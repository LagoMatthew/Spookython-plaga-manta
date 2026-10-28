package graph

import (
	"math/rand/v2"
	"spookython-backend/internal/domain"
)

var MantaParishes = []struct {
	Name       string
	Percentage float64
}{
	{"TARQUI", 0.38},
	{"ELOY_ALFARO", 0.22},
	{"MANTA_CENTRO", 0.18},
	{"LOS_ESTEROS", 0.16},
	{"SAN_MATEO", 0.06},
}

func BuildMantaGraph(popSize int, seed int64) *domain.Graph {
	rng := rand.New(rand.NewPCG(uint64(seed), 101))

	nodes := make([]domain.Node, popSize)
	adj := make([][]domain.Edge, popSize)
	parishNodes := make(map[string][]int)

	// 1. Asignar ciudadanos a parroquias según proporción INEC Manta
	idx := 0
	for i, p := range MantaParishes {
		count := int(float64(popSize) * p.Percentage)
		if i == len(MantaParishes)-1 {
			count = popSize - idx
		}
		for c := 0; c < count && idx < popSize; c++ {
			nodes[idx] = domain.Node{
				ID:             idx,
				Parish:         p.Name,
				State:          domain.StateSusceptible,
				Susceptibility: 1.0,
			}
			parishNodes[p.Name] = append(parishNodes[p.Name], idx)
			idx++
		}
	}

	addUndirectedEdge := func(u, v int, weight float64, layer domain.EdgeLayer) {
		if u == v {
			return
		}
		adj[u] = append(adj[u], domain.Edge{TargetID: v, Weight: weight, Layer: layer})
		adj[v] = append(adj[v], domain.Edge{TargetID: u, Weight: weight, Layer: layer})
		nodes[u].WeightedDegree += weight
		nodes[v].WeightedDegree += weight
	}

	// 2. Capa HOGARES (Cliques de 3 a 5 familiares dentro de la misma parroquia, w = 0.50)
	for _, pList := range parishNodes {
		for i := 0; i < len(pList); {
			houseSize := 3 + rng.IntN(3)
			end := i + houseSize
			if end > len(pList) {
				end = len(pList)
			}
			for a := i; a < end; a++ {
				for b := a + 1; b < end; b++ {
					addUndirectedEdge(pList[a], pList[b], 0.50, domain.LayerHome)
				}
			}
			i = end
		}
	}

	// 3. Capa MERCADOS / HUBS COMERCIALES (ej. Playita Mía en Tarqui, Mercado Central, w = 0.35)
	for _, pList := range parishNodes {
		if len(pList) < 10 {
			continue
		}
		numVendors := max(2, len(pList)/120)
		for v := 0; v < numVendors; v++ {
			hubNode := pList[rng.IntN(len(pList))]
			customers := 15 + rng.IntN(15)
			for c := 0; c < customers; c++ {
				buyer := pList[rng.IntN(len(pList))]
				addUndirectedEdge(hubNode, buyer, 0.35, domain.LayerMarket)
			}
		}
	}

	// 4. Capa COLEGIOS Y ULEAM (Concentrado en Eloy Alfaro y Manta Centro pero recibe de todas las parroquias, w = 0.25)
	numClassrooms := max(5, popSize/150)
	for cl := 0; cl < numClassrooms; cl++ {
		rep := rng.IntN(popSize)
		students := 12 + rng.IntN(10)
		for s := 0; s < students; s++ {
			student := rng.IntN(popSize)
			addUndirectedEdge(rep, student, 0.25, domain.LayerSchool)
		}
	}

	// 5. Capa BUSES URBANOS (Puentes entre Los Esteros, Tarqui, Centro y Eloy Alfaro, w = 0.15)
	busConnections := popSize * 2
	for b := 0; b < busConnections; b++ {
		u := rng.IntN(popSize)
		v := rng.IntN(popSize)
		addUndirectedEdge(u, v, 0.15, domain.LayerBus)
	}

	return &domain.Graph{
		Nodes:     nodes,
		Adjacency: adj,
	}
}
