package domain

type NodeState int

const (
	StateSusceptible NodeState = iota // 🟢 Sano (S)
	StateExposed                      // 🟡 Mordido / Incubando (E)
	StateInfectious                   // 🔴 Zombie / Contagiando (I)
	StateRecovered                    // 🔵 Exorcizado / Inmune (R)
)

type EdgeLayer string

const (
	LayerHome   EdgeLayer = "HOME"   // Hogar (w = 0.50)
	LayerMarket EdgeLayer = "MARKET" // Mercado / Hub comercial (w = 0.35)
	LayerSchool EdgeLayer = "SCHOOL" // Colegios / ULEAM (w = 0.25)
	LayerBus    EdgeLayer = "BUS"    // Buses interparroquiales (w = 0.15)
)

type Edge struct {
	TargetID int
	Weight   float64
	Layer    EdgeLayer
}

type Node struct {
	ID             int
	Parish         string
	State          NodeState
	DaysInState    int
	Vaccinated     bool
	Susceptibility float64 // 1.0 normal, reducido si vacuna parcial
	WeightedDegree float64
}

type Graph struct {
	Nodes     []Node
	Adjacency [][]Edge
}

func (g *Graph) CloneNodes() []Node {
	cloned := make([]Node, len(g.Nodes))
	copy(cloned, g.Nodes)
	return cloned
}

// SimulationRequest es el JSON exacto que envía el Frontend de Gia
type SimulationRequest struct {
	Seed                 int64   `json:"seed"`
	Runs                 int     `json:"runs"`
	Days                 int     `json:"days"`
	PopulationSize       int     `json:"populationSize"`
	PatientZeroParish    string  `json:"patientZeroParish"`
	Beta                 float64 `json:"beta"`
	IncubationDays       int     `json:"incubationDays"`
	InfectiousDays       int     `json:"infectiousDays"`
	DailyBudget          float64 `json:"dailyBudget"`
	VaccineCost          float64 `json:"vaccineCost"`
	VaccineEffectiveness float64 `json:"vaccineEffectiveness"`
	HubClosureCost       float64 `json:"hubClosureCost"`
}

func (req *SimulationRequest) ApplyDefaults() {
	if req.Seed == 0 {
		req.Seed = 20261028
	}
	if req.Runs <= 0 || req.Runs > 500 {
		req.Runs = 100
	}
	if req.Days <= 0 || req.Days > 180 {
		req.Days = 60
	}
	if req.PopulationSize <= 100 || req.PopulationSize > 10000 {
		req.PopulationSize = 2500
	}
	if req.PatientZeroParish == "" {
		req.PatientZeroParish = "TARQUI"
	}
	if req.Beta <= 0 {
		req.Beta = 0.28
	}
	if req.IncubationDays <= 0 {
		req.IncubationDays = 3
	}
	if req.InfectiousDays <= 0 {
		req.InfectiousDays = 5
	}
	if req.DailyBudget <= 0 {
		req.DailyBudget = 350
	}
	if req.VaccineCost <= 0 {
		req.VaccineCost = 10
	}
	if req.VaccineEffectiveness <= 0 || req.VaccineEffectiveness > 1.0 {
		req.VaccineEffectiveness = 0.85
	}
	if req.HubClosureCost <= 0 {
		req.HubClosureCost = 150
	}
}

type DailyPoint struct {
	Day  int `json:"day"`
	S    int `json:"S"`
	E    int `json:"E"`
	I    int `json:"I"`
	R    int `json:"R"`
	IP10 int `json:"I_p10"`
	IP90 int `json:"I_p90"`
}

type ParishCounts struct {
	S int `json:"S"`
	E int `json:"E"`
	I int `json:"I"`
	R int `json:"R"`
}

type ParishDailyBreakdown struct {
	Day      int                     `json:"day"`
	Parishes map[string]ParishCounts `json:"parishes"`
}

type StrategyResult struct {
	TotalInfectedAvg     int                    `json:"totalInfectedAvg"`
	TotalInfectedP10     int                    `json:"totalInfectedP10"`
	TotalInfectedP90     int                    `json:"totalInfectedP90"`
	PeakActiveInfected   int                    `json:"peakActiveInfected"`
	TotalBudgetSpentAvg  float64                `json:"totalBudgetSpentAvg"`
	VaccinesAppliedAvg   int                    `json:"vaccinesAppliedAvg"`
	HubClosuresDaysAvg   int                    `json:"hubClosuresDaysAvg"`
	DailyCurve           []DailyPoint           `json:"dailyCurve"`
	ParishBreakdownByDay []ParishDailyBreakdown `json:"parishBreakdownByDay,omitempty"`
}

// SimulationResponse es el JSON exacto que recibe el Frontend de Gia
type SimulationResponse struct {
	Meta struct {
		Seed            int64  `json:"seed"`
		Runs            int    `json:"runs"`
		PopulationSize  int    `json:"populationSize"`
		ExecutionTimeMs int64  `json:"executionTimeMs"`
		EngineVersion   string `json:"engineVersion"`
	} `json:"meta"`
	Summary struct {
		SavedVsNone      int     `json:"savedVsNone"`
		SavedVsRandom    int     `json:"savedVsRandom"`
		PeakReductionPct float64 `json:"peakReductionPct"`
		TotalCostSpent   float64 `json:"totalCostSpent"`
		CostPerLifeSaved float64 `json:"costPerLifeSaved"`
		MayorReport      string  `json:"mayorReport"`
	} `json:"summary"`
	Strategies map[string]StrategyResult `json:"strategies"`
}
