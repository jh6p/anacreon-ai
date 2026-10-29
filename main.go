package main

import (
	"fmt"
	"os"
	"strings"

	"anacreon-ai/savefile"
)

// ResourceType identifies which resource array to inspect.
type ResourceType string

const (
	ResourceTypeIndus ResourceType = "indus"
	ResourceTypeCargo ResourceType = "cargo"
	ResourceTypeDefns ResourceType = "defns"
	ResourceTypeShips ResourceType = "ships"
)

// ResourceResult holds the planets with the most and least of a given resource.
type ResourceResult struct {
	MaxPlanet *savefile.IndexedRecord[savefile.PlanetRecord]
	MaxValue  uint16
	MinPlanet *savefile.IndexedRecord[savefile.PlanetRecord]
	MinValue  uint16
}

// FindResourceExtremes finds the planet with the most and least of a given resource array
// and index. resourceType: "indus" (0-8), "cargo" (0-6), "defns" (0-3), "ships" (0-6).
func FindResourceExtremes(save *savefile.SaveFile, resourceType ResourceType, index int) (*ResourceResult, error) {
	if index < 0 {
		return nil, fmt.Errorf("index must be non-negative, got %d", index)
	}

	var maxVal, minVal uint16
	var maxP, minP *savefile.IndexedRecord[savefile.PlanetRecord]
	first := true

	for i := range save.Planets {
		p := &save.Planets[i]
		var val uint16

		switch resourceType {
		case ResourceTypeIndus:
			if index >= len(p.Record.Indus) {
				return nil, fmt.Errorf("indus index %d out of range [0-%d]", index, len(p.Record.Indus)-1)
			}
			val = p.Record.Indus[index]
		case ResourceTypeCargo:
			if index >= len(p.Record.Cargo) {
				return nil, fmt.Errorf("cargo index %d out of range [0-%d]", index, len(p.Record.Cargo)-1)
			}
			val = p.Record.Cargo[index]
		case ResourceTypeDefns:
			if index >= len(p.Record.Defns) {
				return nil, fmt.Errorf("defns index %d out of range [0-%d]", index, len(p.Record.Defns)-1)
			}
			val = p.Record.Defns[index]
		case ResourceTypeShips:
			if index >= len(p.Record.Ships) {
				return nil, fmt.Errorf("ships index %d out of range [0-%d]", index, len(p.Record.Ships)-1)
			}
			val = p.Record.Ships[index]
		default:
			return nil, fmt.Errorf("unknown resource type %q (valid: indus, cargo, defns, ships)", resourceType)
		}

		if first {
			maxVal, minVal = val, val
			maxP, minP = &save.Planets[i], &save.Planets[i]
			first = false
		} else {
			if val > maxVal {
				maxVal = val
				maxP = &save.Planets[i]
			}
			if val < minVal {
				minVal = val
				minP = &save.Planets[i]
			}
		}
	}

	if first {
		return &ResourceResult{}, nil
	}

	return &ResourceResult{
		MaxPlanet: maxP,
		MaxValue:  maxVal,
		MinPlanet: minP,
		MinValue:  minVal,
	}, nil
}

// PlanetCoord returns the (x, y) coordinates of a planet.
func PlanetCoord(p *savefile.IndexedRecord[savefile.PlanetRecord]) (int, int) {
	x := int(p.Record.XYX)
	y := int(p.Record.XYY)
	return x, y
}

// FindPlanetName looks up a planet's name by coordinates from the empire name records.
func FindPlanetName(save *savefile.SaveFile, px, py int) string {
	for i := range save.EmpireData {
		emp := &save.EmpireData[i]
		for j := range emp.Names {
			n := &emp.Names[j]
			if int(n.CoordX) == px && int(n.CoordY) == py {
				return n.Name
			}
		}
	}
	return ""
}

// FindPlanetCoord looks up a planet's (x, y) coordinates by name from the given empire's name records.
func FindPlanetCoord(save *savefile.SaveFile, empIdx int, name string) (int, int, bool) {
	if empIdx < 0 || empIdx >= len(save.EmpireData) {
		return 0, 0, false
	}
	emp := &save.EmpireData[empIdx]
	for j := range emp.Names {
		n := &emp.Names[j]
		if strings.EqualFold(n.Name, name) {
			return int(n.CoordX), int(n.CoordY), true
		}
	}
	return 0, 0, false
}

// FindEmpireIndexByName returns the empire index (0-based) matching the given name, or -1 if not found.
func FindEmpireIndexByName(save *savefile.SaveFile, name string) int {
	for i, emp := range save.EmpireData {
		if strings.EqualFold(emp.EmpireName, name) {
			return i
		}
	}
	return -1
}

// PrintPlanetsForEmpire prints all planets owned by the empire with the given name.
func PrintPlanetsForEmpire(save *savefile.SaveFile, empireName string) {
	empIdx := FindEmpireIndexByName(save, empireName)
	if empIdx < 0 {
		fmt.Printf("No empire found with name %q\n", empireName)
		return
	}

	emp := save.EmpireData[empIdx]
	fmt.Printf("\nPlanets for %s (Empire%d):\n", emp.EmpireName, empIdx+1)

	count := 0
	for i := range save.Planets {
		p := &save.Planets[i]
		if int(p.Record.Emp) != empIdx {
			continue
		}
		count++
		x, y := PlanetCoord(p)
		name := FindPlanetName(save, x, y)
		fmt.Printf("  %d. (%d,%d) %q Emp=%d Pop=%d Tech=%d Eff=%d\n",
			count, x, y, name, p.Record.Emp, p.Record.Pop, p.Record.Tech, p.Record.Eff)
	}

	if count == 0 {
		fmt.Printf("  (no planets found)\n")
	}
}

// PrintEmpireSummary prints a summary of empires, optionally filtered by name.
func PrintEmpireSummary(save *savefile.SaveFile, empireName string) {
	for i, emp := range save.EmpireData {
		if !emp.InUse {
			continue
		}
		if empireName != "" && !strings.EqualFold(emp.EmpireName, empireName) {
			continue
		}
		role := "AI"
		if emp.IsAPlayer {
			role = "PLAYER"
		}
		fmt.Printf("Empire%d: role=%s name=%q", i+1, role, emp.EmpireName)
		if len(emp.Names) > 0 {
			fmt.Print(" names=[")
			for j, n := range emp.Names {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Printf("%q", n.Name)
			}
			fmt.Print("]")
		}

		PrintPlanetsForEmpire(save, empireName)
		PrintPlanetsScoutedByEmpire(save, i)
		fmt.Println()
	}
}

// PlanetsScoutedByEmpire returns planets where ScoutedBy has the bit set for empIndex.
// The bit check is: ScoutedBy & (1 << empIndex) != 0.
func PlanetsScoutedByEmpire(save *savefile.SaveFile, empIndex int) ([]*savefile.IndexedRecord[savefile.PlanetRecord], error) {
	if save == nil {
		return nil, fmt.Errorf("save file cannot be nil")
	}
	if empIndex < 0 {
		return nil, fmt.Errorf("empire index must be non-negative, got %d", empIndex)
	}
	if empIndex > 7 {
		return nil, fmt.Errorf("empire index %d out of range for ScoutedBy bitmask [0-7]", empIndex)
	}
	if empIndex >= len(save.EmpireData) {
		return nil, fmt.Errorf("empire index %d out of range for empire data [0-%d]", empIndex, len(save.EmpireData)-1)
	}

	mask := uint8(1 << uint(empIndex))
	planets := make([]*savefile.IndexedRecord[savefile.PlanetRecord], 0)
	for i := range save.Planets {
		p := &save.Planets[i]
		if p.Record.ScoutedBy&mask != 0 {
			planets = append(planets, p)
		}
	}

	return planets, nil
}

// PrintPlanetsScoutedBySecondBit prints all planet names where ScoutedBy has the second bit set (value & 2 != 0).
func PrintPlanetsScoutedByEmpire(save *savefile.SaveFile, empIndex int) {
	fmt.Printf("\nPlanets where ScoutedBy has the bit set for empire index %d:\n", empIndex)
	planets, err := PlanetsScoutedByEmpire(save, empIndex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding scouted planets: %v\n", err)
		return
	}
	if len(planets) == 0 {
		fmt.Println("  (none found)")
		return
	}

	for i := range planets {
		p := planets[i]
		x, y := PlanetCoord(p)
		name := FindPlanetName(save, x, y)
		fmt.Printf("  %d. (%d,%d) %q (ScoutedBy=%d)\n",
			i+1, x, y, name, p.Record.ScoutedBy)
	}
}

func loadSave(path string) (*savefile.SaveFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read save file: %w", err)
	}

	save, err := savefile.ParseSaveFile(content)
	if err != nil {
		return nil, fmt.Errorf("parse save file: %w", err)
	}

	return save, nil
}

func main() {
	path := "C:\\Users\\busin\\Documents\\CDrive\\Anac20\\202505\\J250508A"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	empireName := ""
	if len(os.Args) > 2 {
		empireName = os.Args[2]
	}

	save, err := loadSave(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %q (version=%d)\n", path, save.Version)
	fmt.Printf("Env: year=%d scenario=%q player=%d timePerTurn=%d\n",
		save.Environment.Year,
		save.Environment.ScenarioFile,
		save.Environment.Player,
		save.Environment.TimePerTurn,
	)
	fmt.Printf("Sector: %dx%d\n", save.Sector.SizeX, save.Sector.SizeY)
	fmt.Printf("Planets=%d Starbases=%d Fleets=%d Stargates=%d Constructions=%d\n",
		len(save.Planets), len(save.Starbases), len(save.Fleets), len(save.Stargates), len(save.Constr),
	)
	fmt.Printf("Messages=%d Empires=%d\n", len(save.Messages), len(save.EmpireData))
	fmt.Printf("Player=%d\n", save.Environment.Player)
	for i, emp := range save.EmpireData {
		if !emp.InUse {
			continue
		}
		if empireName != "" && !strings.EqualFold(emp.EmpireName, empireName) {
			continue
		}
		role := "AI"
		if emp.IsAPlayer {
			role = "PLAYER"
		}
		fmt.Printf("Empire%d: role=%s name=%q", i+1, role, emp.EmpireName)
		if len(emp.Names) > 0 {
			fmt.Print(" names=[")
			for j, n := range emp.Names {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Printf("%q", n.Name)
			}
			fmt.Print("]")
		}
		fmt.Println()
	}
	fmt.Printf("NPE sections=%d\n", len(save.NPESaves))
	for empIdx, npe := range save.NPESaves {
		if empireName != "" && !strings.EqualFold(save.EmpireData[empIdx].EmpireName, empireName) {
			continue
		}
		fmt.Printf("  Empire%d: typ=%d raw_len=%d\n", empIdx+1, npe.Typ, len(npe.Raw))
	}

	if metal, err := FindResourceExtremes(save, ResourceTypeIndus, 0); err != nil {
		fmt.Fprintf(os.Stderr, "error finding metal extremes: %v\n", err)
	} else if metal.MaxValue > 0 {
		maxX, maxY := PlanetCoord(metal.MaxPlanet)
		minX, minY := PlanetCoord(metal.MinPlanet)
		fmt.Printf("Metal (Indus[0]): max=%d at (%d,%d) (%s), min=%d at (%d,%d) (%s)\n",
			metal.MaxValue, maxX, maxY, FindPlanetName(save, maxX, maxY),
			metal.MinValue, minX, minY, FindPlanetName(save, minX, minY))
	}

	PrintEmpireSummary(save, empireName)
}
