package strategy

import (
	"math/rand/v2"
	"sort"
	"spookython-backend/internal/domain"
)

type DailyInterventionOutcome struct {
	ClosureActiveInParish map[string]bool
	BudgetSpent           float64
	VaccinesApplied       int
	HubClosureTriggered   bool
}

type InterventionStrategy interface {
	Name() string
	Decide(day int, nodes []domain.Node, g *domain.Graph, req domain.SimulationRequest, rng *rand.Rand) DailyInterventionOutcome
}

// 1. LÍNEA BASE 0: No hacer nada
type NoInterventionStrategy struct{}

func (s *NoInterventionStrategy) Name() string { return "noIntervention" }

func (s *NoInterventionStrategy) Decide(day int, nodes []domain.Node, g *domain.Graph, req domain.SimulationRequest, rng *rand.Rand) DailyInterventionOutcome {
	return DailyInterventionOutcome{
		ClosureActiveInParish: make(map[string]bool),
	}
}

// 2. LÍNEA BASE 1: Actuar al azar con el mismo presupuesto
type RandomInterventionStrategy struct{}

func (s *RandomInterventionStrategy) Name() string { return "randomIntervention" }

func (s *RandomInterventionStrategy) Decide(day int, nodes []domain.Node, g *domain.Graph, req domain.SimulationRequest, rng *rand.Rand) DailyInterventionOutcome {
	if day < 3 {
		return DailyInterventionOutcome{ClosureActiveInParish: make(map[string]bool)}
	}
	outcome := DailyInterventionOutcome{
		ClosureActiveInParish: make(map[string]bool),
	}
	remainingBudget := req.DailyBudget

	// Filtrar SOLO nodos Sanos (Susceptibles) y no vacunados (Regla estricta del jurado)
	var eligible []int
	for i := range nodes {
		if nodes[i].State == domain.StateSusceptible && !nodes[i].Vaccinated {
			eligible = append(eligible, i)
		}
	}

	// Barajar al azar de forma determinista con la semilla
	rng.Shuffle(len(eligible), func(i, j int) {
		eligible[i], eligible[j] = eligible[j], eligible[i]
	})

	for _, idx := range eligible {
		if remainingBudget < req.VaccineCost {
			break
		}
		remainingBudget -= req.VaccineCost
		outcome.BudgetSpent += req.VaccineCost
		outcome.VaccinesApplied++
		applyVaccineToNode(&nodes[idx], req.VaccineEffectiveness, rng)
	}

	return outcome
}

// 3. NUESTRA ESTRATEGIA: Comité de Contención Inteligente (Cortafuegos en Grafo + Cierre Dinámico de Hubs)
type SmartContainmentStrategy struct{}

func (s *SmartContainmentStrategy) Name() string { return "smartContainment" }

func (s *SmartContainmentStrategy) Decide(day int, nodes []domain.Node, g *domain.Graph, req domain.SimulationRequest, rng *rand.Rand) DailyInterventionOutcome {
	if day < 3 {
		return DailyInterventionOutcome{ClosureActiveInParish: make(map[string]bool)}
	}
	outcome := DailyInterventionOutcome{
		ClosureActiveInParish: make(map[string]bool),
	}
	remainingBudget := req.DailyBudget

	// Paso A: Detectar parroquias con brote activo crítico y cerrar temporalmente Mercados/Colegios
	parishActiveCases := make(map[string]int)
	for i := range nodes {
		if nodes[i].State == domain.StateInfectious || nodes[i].State == domain.StateExposed {
			parishActiveCases[nodes[i].Parish]++
		}
	}

	for parish, count := range parishActiveCases {
		if count >= 8 && remainingBudget >= req.HubClosureCost {
			outcome.ClosureActiveInParish[parish] = true
			outcome.HubClosureTriggered = true
			remainingBudget -= req.HubClosureCost
			outcome.BudgetSpent += req.HubClosureCost
		}
	}

	// Paso B: Vacunación Cortafuegos (Anillo de Contención + Centralidad Ponderada)
	type candidate struct {
		id    int
		score float64
	}
	var candidates []candidate

	for i := range nodes {
		// Regla del jurado: JAMÁS vacunar infectados (E/I) ni inmunes (R)
		if nodes[i].State != domain.StateSusceptible || nodes[i].Vaccinated {
			continue
		}

		exposedNeighbors := 0
		zombieNeighbors := 0
		for _, edge := range g.Adjacency[i] {
			nbState := nodes[edge.TargetID].State
			if nbState == domain.StateInfectious {
				zombieNeighbors++
			} else if nbState == domain.StateExposed {
				exposedNeighbors++
			}
		}

		// Priorizar super-spreaders (mercados/buses) que están en el anillo de fuego del brote
		ringUrgency := 1.0 + float64(exposedNeighbors)*4.5 + float64(zombieNeighbors)*8.0
		score := nodes[i].WeightedDegree * ringUrgency
		candidates = append(candidates, candidate{id: i, score: score})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].score > candidates[j].score
	})

	for _, cand := range candidates {
		if remainingBudget < req.VaccineCost {
			break
		}
		remainingBudget -= req.VaccineCost
		outcome.BudgetSpent += req.VaccineCost
		outcome.VaccinesApplied++
		applyVaccineToNode(&nodes[cand.id], req.VaccineEffectiveness, rng)
	}

	return outcome
}

func applyVaccineToNode(node *domain.Node, effectiveness float64, rng *rand.Rand) {
	node.Vaccinated = true
	if rng.Float64() < effectiveness {
		// Inmunidad total inmediata
		node.State = domain.StateRecovered
		node.DaysInState = 0
	} else {
		// Si es una variante resistente (Brote Sorpresa), aún reduce susceptibilidad un 60%
		node.Susceptibility = 0.40
	}
}
