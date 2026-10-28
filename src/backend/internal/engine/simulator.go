package engine

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"spookython-backend/internal/domain"
	"spookython-backend/internal/graph"
	"spookython-backend/internal/strategy"
	"sync"
	"time"
)

type singleRunResult struct {
	dailyS            []int
	dailyE            []int
	dailyI            []int
	dailyR            []int
	parishDaily       []map[string]domain.ParishCounts
	totalEverInfected int
	peakActiveI       int
	budgetSpent       float64
	vaccinesUsed      int
	closureDays       int
}

func RunFullSimulation(req domain.SimulationRequest) domain.SimulationResponse {
	req.ApplyDefaults()
	startTime := time.Now()

	// 1. Generar el grafo multicapa de Manta una vez por semilla
	baseGraph := graph.BuildMantaGraph(req.PopulationSize, req.Seed)

	strategies := []strategy.InterventionStrategy{
		&strategy.NoInterventionStrategy{},
		&strategy.RandomInterventionStrategy{},
		&strategy.SmartContainmentStrategy{},
	}

	resultsMap := make(map[string]domain.StrategyResult)

	for _, strat := range strategies {
		keepParishBreakdown := (strat.Name() == "smartContainment")
		resultsMap[strat.Name()] = runMonteCarloForStrategy(baseGraph, req, strat, keepParishBreakdown)
	}

	noInt := resultsMap["noIntervention"]
	randInt := resultsMap["randomIntervention"]
	smartInt := resultsMap["smartContainment"]

	savedVsNone := max(0, noInt.TotalInfectedAvg-smartInt.TotalInfectedAvg)
	savedVsRandom := max(0, randInt.TotalInfectedAvg-smartInt.TotalInfectedAvg)

	peakRed := 0.0
	if noInt.PeakActiveInfected > 0 {
		peakRed = math.Round((float64(noInt.PeakActiveInfected-smartInt.PeakActiveInfected)/float64(noInt.PeakActiveInfected))*1000) / 10
	}

	costPerLife := 0.0
	if savedVsNone > 0 {
		costPerLife = math.Round((smartInt.TotalBudgetSpentAvg/float64(savedVsNone))*100) / 100
	}

	var res domain.SimulationResponse
	res.Meta.Seed = req.Seed
	res.Meta.Runs = req.Runs
	res.Meta.PopulationSize = req.PopulationSize
	res.Meta.ExecutionTimeMs = time.Since(startTime).Milliseconds()
	res.Meta.EngineVersion = "SEIR-Manta-Graph-v1.0"

	res.Summary.SavedVsNone = savedVsNone
	res.Summary.SavedVsRandom = savedVsRandom
	res.Summary.PeakReductionPct = peakRed
	res.Summary.TotalCostSpent = math.Round(smartInt.TotalBudgetSpentAvg)
	res.Summary.CostPerLifeSaved = costPerLife
	res.Summary.MayorReport = fmt.Sprintf(
		"Señor Alcalde de Manta: Ante el brote iniciado en la parroquia %s (Semilla %d, β=%.2f, efectividad vacunal %.0f%%), el Comité de Contención recomienda activar cierres tácticos de mercados/hubs durante %d días críticos y aplicar %d dosis en anillo sobre corredores de transporte y comercio. Esta estrategia evita %d contagios frente a no intervenir (%d más que vacunar al azar), reduce el pico hospitalario en un %.1f%% y optimiza el presupuesto con un costo de $%.2f por ciudadano protegido.",
		req.PatientZeroParish, req.Seed, req.Beta, req.VaccineEffectiveness*100,
		smartInt.HubClosuresDaysAvg, smartInt.VaccinesAppliedAvg,
		savedVsNone, savedVsRandom, peakRed, costPerLife,
	)

	res.Strategies = resultsMap
	return res
}

func runMonteCarloForStrategy(g *domain.Graph, req domain.SimulationRequest, strat strategy.InterventionStrategy, recordParishes bool) domain.StrategyResult {
	runs := req.Runs
	allRuns := make([]singleRunResult, runs)

	var wg sync.WaitGroup
	wg.Add(runs)

	for r := 0; r < runs; r++ {
		go func(runIdx int) {
			defer wg.Done()
			// Semilla determinista única para cada corrida (garantiza reproducibilidad exacta)
			runRng := rand.New(rand.NewPCG(uint64(req.Seed), uint64(runIdx+1000)))
			allRuns[runIdx] = simulateSingleRun(g, req, strat, runRng, recordParishes && runIdx == 0)
		}(r)
	}
	wg.Wait()

	// Agregar promedios y percentiles P10 y P90 día por día
	dailyCurve := make([]domain.DailyPoint, req.Days+1)
	for day := 0; day <= req.Days; day++ {
		sumS, sumE, sumI, sumR := 0, 0, 0, 0
		iValues := make([]int, runs)
		for r := 0; r < runs; r++ {
			sumS += allRuns[r].dailyS[day]
			sumE += allRuns[r].dailyE[day]
			sumI += allRuns[r].dailyI[day]
			sumR += allRuns[r].dailyR[day]
			iValues[r] = allRuns[r].dailyI[day]
		}
		sort.Ints(iValues)
		p10Idx := int(float64(runs) * 0.10)
		p90Idx := int(float64(runs) * 0.90)
		if p90Idx >= runs {
			p90Idx = runs - 1
		}

		dailyCurve[day] = domain.DailyPoint{
			Day:  day,
			S:    sumS / runs,
			E:    sumE / runs,
			I:    sumI / runs,
			R:    sumR / runs,
			IP10: iValues[p10Idx],
			IP90: iValues[p90Idx],
		}
	}

	totals := make([]int, runs)
	sumTotalInf, sumPeak, sumVaccines, sumClosures := 0, 0, 0, 0
	sumBudget := 0.0
	for r := 0; r < runs; r++ {
		totals[r] = allRuns[r].totalEverInfected
		sumTotalInf += allRuns[r].totalEverInfected
		sumPeak += allRuns[r].peakActiveI
		sumBudget += allRuns[r].budgetSpent
		sumVaccines += allRuns[r].vaccinesUsed
		sumClosures += allRuns[r].closureDays
	}
	sort.Ints(totals)

	res := domain.StrategyResult{
		TotalInfectedAvg:    sumTotalInf / runs,
		TotalInfectedP10:    totals[int(float64(runs)*0.10)],
		TotalInfectedP90:    totals[min(runs-1, int(float64(runs)*0.90))],
		PeakActiveInfected:  sumPeak / runs,
		TotalBudgetSpentAvg: sumBudget / float64(runs),
		VaccinesAppliedAvg:  sumVaccines / runs,
		HubClosuresDaysAvg:  sumClosures / runs,
		DailyCurve:          dailyCurve,
	}

	if recordParishes && len(allRuns[0].parishDaily) > 0 {
		breakdown := make([]domain.ParishDailyBreakdown, req.Days+1)
		for day := 0; day <= req.Days; day++ {
			breakdown[day] = domain.ParishDailyBreakdown{
				Day:      day,
				Parishes: allRuns[0].parishDaily[day],
			}
		}
		res.ParishBreakdownByDay = breakdown
	}

	return res
}

func simulateSingleRun(g *domain.Graph, req domain.SimulationRequest, strat strategy.InterventionStrategy, rng *rand.Rand, trackParishes bool) singleRunResult {
	nodes := g.CloneNodes()
	popSize := len(nodes)

	// Seleccionar primeros 3 casos (Paciente Cero + cluster inicial) en la parroquia elegida
	var parishCandidates []int
	for i := range nodes {
		if nodes[i].Parish == req.PatientZeroParish {
			parishCandidates = append(parishCandidates, i)
		}
	}
	if len(parishCandidates) == 0 {
		parishCandidates = []int{0, 1, 2}
	}

	initialCases := min(3, len(parishCandidates))
	totalEverInfected := 0
	for c := 0; c < initialCases; c++ {
		chosen := parishCandidates[rng.IntN(len(parishCandidates))]
		if nodes[chosen].State == domain.StateSusceptible {
			nodes[chosen].State = domain.StateInfectious
			totalEverInfected++
		}
	}

	out := singleRunResult{
		dailyS: make([]int, req.Days+1),
		dailyE: make([]int, req.Days+1),
		dailyI: make([]int, req.Days+1),
		dailyR: make([]int, req.Days+1),
	}
	if trackParishes {
		out.parishDaily = make([]map[string]domain.ParishCounts, req.Days+1)
	}

	for day := 0; day <= req.Days; day++ {
		// 1. Contar estados actuales del día
		cntS, cntE, cntI, cntR := 0, 0, 0, 0
		var pMap map[string]domain.ParishCounts
		if trackParishes {
			pMap = make(map[string]domain.ParishCounts)
		}

		for i := range nodes {
			switch nodes[i].State {
			case domain.StateSusceptible:
				cntS++
			case domain.StateExposed:
				cntE++
			case domain.StateInfectious:
				cntI++
			case domain.StateRecovered:
				cntR++
			}
			if trackParishes {
				pc := pMap[nodes[i].Parish]
				switch nodes[i].State {
				case domain.StateSusceptible:
					pc.S++
				case domain.StateExposed:
					pc.E++
				case domain.StateInfectious:
					pc.I++
				case domain.StateRecovered:
					pc.R++
				}
				pMap[nodes[i].Parish] = pc
			}
		}

		out.dailyS[day] = cntS
		out.dailyE[day] = cntE
		out.dailyI[day] = cntI
		out.dailyR[day] = cntR
		if cntI > out.peakActiveI {
			out.peakActiveI = cntI
		}
		if trackParishes {
			out.parishDaily[day] = pMap
		}

		if day == req.Days || (cntE == 0 && cntI == 0) {
			// Si la plaga ya se extinguió, rellenar el resto de días estables
			for rem := day + 1; rem <= req.Days; rem++ {
				out.dailyS[rem] = cntS
				out.dailyE[rem] = 0
				out.dailyI[rem] = 0
				out.dailyR[rem] = cntR
				if trackParishes {
					out.parishDaily[rem] = pMap
				}
			}
			break
		}

		// 2. Ejecutar decisión diaria de la estrategia (Vacunas + Cierres de Hubs)
		decision := strat.Decide(day, nodes, g, req, rng)
		out.budgetSpent += decision.BudgetSpent
		out.vaccinesUsed += decision.VaccinesApplied
		if decision.HubClosureTriggered {
			out.closureDays++
		}

		// 3. Calcular contagios por aristas activas: P(S -> E) = 1 - prod(1 - beta * w_ij * mod)
		newExposed := make([]bool, popSize)
		for i := range nodes {
			if nodes[i].State != domain.StateSusceptible {
				continue
			}
			probNotInfected := 1.0
			for _, edge := range g.Adjacency[i] {
				if nodes[edge.TargetID].State != domain.StateInfectious {
					continue
				}
				effectiveWeight := edge.Weight
				// Si hay cierre de mercados/colegios activo en la parroquia, reduce el peso 80%
				if decision.ClosureActiveInParish[nodes[i].Parish] && (edge.Layer == domain.LayerMarket || edge.Layer == domain.LayerSchool) {
					effectiveWeight *= 0.20
				}
				pEdge := req.Beta * effectiveWeight * nodes[i].Susceptibility
				if pEdge > 0.95 {
					pEdge = 0.95
				}
				probNotInfected *= (1.0 - pEdge)
			}
			probInfection := 1.0 - probNotInfected
			if probInfection > 0 && rng.Float64() < probInfection {
				newExposed[i] = true
			}
		}

		// 4. Transiciones diarias del modelo SEIR
		for i := range nodes {
			switch nodes[i].State {
			case domain.StateSusceptible:
				if newExposed[i] {
					nodes[i].State = domain.StateExposed
					nodes[i].DaysInState = 0
					totalEverInfected++
				}
			case domain.StateExposed:
				nodes[i].DaysInState++
				if nodes[i].DaysInState >= req.IncubationDays {
					nodes[i].State = domain.StateInfectious
					nodes[i].DaysInState = 0
				}
			case domain.StateInfectious:
				nodes[i].DaysInState++
				if nodes[i].DaysInState >= req.InfectiousDays {
					nodes[i].State = domain.StateRecovered
					nodes[i].DaysInState = 0
				}
			}
		}
	}

	out.totalEverInfected = totalEverInfected
	return out
}
