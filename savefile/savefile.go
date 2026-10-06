package savefile

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	headerSignatureSize = 32
	currentSFVersion    = 15

	maxPlanets      = 200
	maxStarbases    = 100
	maxFleets       = 240
	maxStargates    = 50
	maxConstrSites  = 50
	maxEmpires      = 8
	maxEmpireWithAI = 9 // Empire1..Empire8 + Indep

	// Verified against the Turbo Pascal v2.0 save format used by LOADSAVE.PAS.
	sectorRecordSize        = 5
	planetRecordSizeV15     = 90
	starbaseRecordSize      = 91
	fleetRecordSize         = 57
	commandRecordSize       = 5
	stargateRecordSizeV15   = 28
	constrRecordSize        = 9
	messageRecordSize       = 23
	lineStrSize             = 81 // STRING[80]
	empireDataRecordSize    = 183
	nameRecordSize          = 17
	newsRecordSize          = 15
	npeDataRecordSize       = 5
	npePirateDataSize       = 739
	npeKingdomDataSize      = 454
	npeBerserkerDataSize    = 930
	npeGuardianDataSize     = 430
	scenarioFilenameMaxChar = 16
)

type SaveFile struct {
	Signature string
	Version   uint16

	Environment EnvironmentSection
	Sector      SectorSection
	Planets     []IndexedRecord[PlanetRecord]
	Starbases   []IndexedRecord[StarbaseRecord]
	Fleets      []FleetEntry
	Stargates   []IndexedRecord[StargateRecord]
	Constr      []IndexedRecord[ConstrRecord]
	Messages    []Message
	EmpireData  []EmpireData
	NewsByEmp   [maxEmpires][]NewsRecord
	NPEData     [maxEmpireWithAI]NPEDataRecord
	NPESaves    map[uint8]NPEPayload // keyed by empire index [0..7] for Empire1..Empire8
}

type EnvironmentSection struct {
	Year          uint16
	Player        uint8
	EmpiresToMove uint16
	ScenarioFile  string
	TimePerTurn   uint16
	AutoSave      bool
	AsyncTurns    bool
	PauseActive   bool
	ReEnterGame   bool
}

type SectorSection struct {
	SizeX uint16
	SizeY uint16
	Rows  [][]SectorRecord // [y][x]
}

type SectorRecord struct {
	ObjType   uint8
	ObjIndex  uint8
	Flts      uint8
	MineScout uint8
	Special   uint8
}

type IndexedRecord[T any] struct {
	Index  uint16
	Record T
}

type PlanetRecord struct {
	XYX, XYY        uint8
	Emp             uint8
	ScoutedBy       uint8
	KnownBy         uint8
	Cls             uint8
	Typ             uint8
	ImpExp          int16
	Tech            uint8
	Eff             uint8
	RevIndex        uint8
	Special         uint8
	TerraformTarget uint8
	Pop             uint16
	Ships           [7]uint16
	Cargo           [7]uint16
	Defns           [4]uint16
	Indus           [9]uint16
	TriReserve      uint16
	Reserved        [16]byte
	NextObjType     uint8
	NextObjIndex    uint8
}

type StarbaseRecord struct {
	XYX, XYY     uint8
	Emp          uint8
	ScoutedBy    uint8
	KnownBy      uint8
	STyp         uint8
	Typ          uint8
	Tech         uint8
	Eff          uint8
	RevIndex     uint8
	Special      uint8
	Pop          uint16
	Ships        [7]uint16
	Cargo        [7]uint16
	Defns        [4]uint16
	Indus        [9]uint16
	Move         uint8
	DestX, DestY uint8
	Status       uint8
	Reserved     [18]byte
	NextObjType  uint8
	NextObjIndex uint8
}

type FleetRecord struct {
	XYX, XYY     uint8
	Emp          uint8
	ScoutedBy    uint8
	Ships        [7]uint16
	Cargo        [7]uint16
	DestX, DestY uint8
	Status       uint8
	FuelHigh     uint8
	Fuel         int16
	KnownBy      uint8
	NextOrder    uint8
	OrderData    [6]byte
	NPEDataIndex uint8
	Reserved     [8]byte
	NextObjType  uint8
	NextObjIndex uint8
}

type FleetEntry struct {
	Index    uint16
	Record   FleetRecord
	Commands []CommandRecord
}

type CommandRecord struct {
	Typ    uint8
	RawArg [4]byte
}

type StargateRecord struct {
	XYX, XYY     uint8
	Emp          uint8
	ScoutedBy    uint8
	KnownBy      uint8
	GTyp         uint8
	DestX, DestY uint8
	WLF          [9]uint16 // Empire1..Indep
	NextObjType  uint8
	NextObjIndex uint8
}

type ConstrRecord struct {
	XYX, XYY         uint8
	Emp              uint8
	ScoutedBy        uint8
	KnownBy          uint8
	CTyp             uint8
	TimeToCompletion uint8
	NextObjType      uint8
	NextObjIndex     uint8
}

type Message struct {
	Sender      uint8
	Recipient   uint8
	ReadBy      uint8
	Read        bool
	Intercepted bool
	NoOfLines   uint8
	Lines       []string
	RawRecord   [messageRecordSize]byte
}

type EmpireData struct {
	InUse           bool
	IsAPlayer       bool
	EmpireName      string
	Pass            string
	TimeLeft        int16
	CapitalObjType  uint8
	CapitalObjIndex uint8
	TechnologyLevel uint8
	TechnologySet   uint32
	IsAnEmpress     bool
	RevFactor       int16
	Founding        uint16
	Modifiers       uint8
	Names           []NameRecord
	RawRecord       [empireDataRecordSize]byte
}

type NameRecord struct {
	Name       string
	CoordX     uint8
	CoordY     uint8
	IDObjType  uint8
	IDObjIndex uint8
	RawNextPtr uint32
	RawRecord  [nameRecordSize]byte
}

type NewsRecord struct {
	Headline   uint8
	LocX       uint8
	LocY       uint8
	LocObjType uint8
	LocObjIdx  uint8
	Parm1      int16
	Parm2      int16
	Parm3      int16
	RawNextPtr uint32
	RawRecord  [newsRecordSize]byte
}

type NPEDataRecord struct {
	Typ        uint8
	RawDataPtr uint32
}

type NPEPayload struct {
	EmpireIndex uint8
	Typ         uint8
	Raw         []byte
}

type parser struct {
	data []byte
	off  int
}

func ParseSaveFile(data []byte) (*SaveFile, error) {
	p := &parser{data: data}
	out := &SaveFile{
		NPESaves: make(map[uint8]NPEPayload),
	}

	sig, err := p.readFixedString(headerSignatureSize)
	if err != nil {
		return nil, fmt.Errorf("header signature: %w", err)
	}
	out.Signature = sig

	out.Version, err = p.u16()
	if err != nil {
		return nil, fmt.Errorf("header version: %w", err)
	}
	if out.Version > currentSFVersion {
		return nil, fmt.Errorf("unsupported save version %d (current parser supports up to %d)", out.Version, currentSFVersion)
	}

	out.Environment, err = p.readEnvironment()
	if err != nil {
		return nil, fmt.Errorf("environment: %w", err)
	}

	out.Sector, err = p.readSector()
	if err != nil {
		return nil, fmt.Errorf("sector: %w", err)
	}

	out.Planets, err = p.readPlanets(out.Version)
	if err != nil {
		return nil, fmt.Errorf("planets: %w", err)
	}

	out.Starbases, err = p.readStarbases()
	if err != nil {
		return nil, fmt.Errorf("starbases: %w", err)
	}

	out.Fleets, err = p.readFleets()
	if err != nil {
		return nil, fmt.Errorf("fleets: %w", err)
	}

	out.Stargates, err = p.readStargates(out.Version)
	if err != nil {
		return nil, fmt.Errorf("stargates: %w", err)
	}

	out.Constr, err = p.readConstr()
	if err != nil {
		return nil, fmt.Errorf("constructions: %w", err)
	}

	out.Messages, err = p.readMessages()
	if err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}

	out.EmpireData, err = p.readEmpireData()
	if err != nil {
		return nil, fmt.Errorf("empire data: %w", err)
	}

	out.NewsByEmp, err = p.readNews()
	if err != nil {
		return nil, fmt.Errorf("news data: %w", err)
	}

	out.NPEData, err = p.readNPEDataArray()
	if err != nil {
		return nil, fmt.Errorf("npe data array: %w", err)
	}

	if err := p.readNPESubsections(out); err != nil {
		return nil, fmt.Errorf("npe subsection data: %w", err)
	}

	if p.remaining() != 0 {
		return nil, fmt.Errorf("trailing bytes after parse: %d", p.remaining())
	}

	return out, nil
}

func (p *parser) readEnvironment() (EnvironmentSection, error) {
	var out EnvironmentSection
	var err error

	out.Year, err = p.u16()
	if err != nil {
		return out, err
	}
	out.Player, err = p.u8()
	if err != nil {
		return out, err
	}
	out.EmpiresToMove, err = p.u16()
	if err != nil {
		return out, err
	}
	out.ScenarioFile, err = p.readPascalString(scenarioFilenameMaxChar)
	if err != nil {
		return out, err
	}
	out.TimePerTurn, err = p.u16()
	if err != nil {
		return out, err
	}
	out.AutoSave, err = p.bool()
	if err != nil {
		return out, err
	}
	out.AsyncTurns, err = p.bool()
	if err != nil {
		return out, err
	}
	out.PauseActive, err = p.bool()
	if err != nil {
		return out, err
	}
	out.ReEnterGame, err = p.bool()
	if err != nil {
		return out, err
	}

	return out, nil
}

func (p *parser) readSector() (SectorSection, error) {
	var out SectorSection
	var err error
	out.SizeX, err = p.u16()
	if err != nil {
		return out, err
	}
	out.SizeY, err = p.u16()
	if err != nil {
		return out, err
	}

	out.Rows = make([][]SectorRecord, int(out.SizeY)+1)
	for y := 0; y <= int(out.SizeY); y++ {
		row := make([]SectorRecord, int(out.SizeX)+1)
		for x := 0; x <= int(out.SizeX); x++ {
			raw, err := p.readBytes(sectorRecordSize)
			if err != nil {
				return out, fmt.Errorf("sector row=%d col=%d: %w", y, x, err)
			}
			row[x] = SectorRecord{
				ObjType:   raw[0],
				ObjIndex:  raw[1],
				Flts:      raw[2],
				MineScout: raw[3],
				Special:   raw[4],
			}
		}
		out.Rows[y] = row
	}

	return out, nil
}

func (p *parser) readPlanets(version uint16) ([]IndexedRecord[PlanetRecord], error) {
	recSize := planetRecordSizeV15
	if version <= 13 {
		recSize = planetRecordSizeV15 - 1 // no TerraformTarget in older versions
	}
	return readIndexed(p, maxPlanets, recSize, parsePlanetRecordVersion(version))
}

func parsePlanetRecordVersion(version uint16) func([]byte) (PlanetRecord, error) {
	return func(raw []byte) (PlanetRecord, error) {
		var pr PlanetRecord
		r := &parser{data: raw}
		var err error
		pr.XYX, err = r.u8()
		if err != nil {
			return pr, err
		}
		pr.XYY, err = r.u8()
		if err != nil {
			return pr, err
		}
		pr.Emp, _ = r.u8()
		pr.ScoutedBy, _ = r.u8()
		pr.KnownBy, _ = r.u8()
		pr.Cls, _ = r.u8()
		pr.Typ, _ = r.u8()
		imp, _ := r.u16()
		pr.ImpExp = int16(imp)
		pr.Tech, _ = r.u8()
		pr.Eff, _ = r.u8()
		pr.RevIndex, _ = r.u8()
		pr.Special, _ = r.u8()
		if version > 13 {
			pr.TerraformTarget, _ = r.u8()
		}
		pr.Pop, _ = r.u16()
		for i := range pr.Ships {
			pr.Ships[i], _ = r.u16()
		}
		for i := range pr.Cargo {
			pr.Cargo[i], _ = r.u16()
		}
		for i := range pr.Defns {
			pr.Defns[i], _ = r.u16()
		}
		for i := range pr.Indus {
			pr.Indus[i], _ = r.u16()
		}
		pr.TriReserve, _ = r.u16()
		reserved, err := r.readBytes(len(pr.Reserved))
		if err != nil {
			return pr, err
		}
		copy(pr.Reserved[:], reserved)
		pr.NextObjType, _ = r.u8()
		pr.NextObjIndex, _ = r.u8()
		return pr, nil
	}
}

func (p *parser) readStarbases() ([]IndexedRecord[StarbaseRecord], error) {
	return readIndexed(p, maxStarbases, starbaseRecordSize, parseStarbaseRecord)
}

func parseStarbaseRecord(raw []byte) (StarbaseRecord, error) {
	var out StarbaseRecord
	r := &parser{data: raw}
	out.XYX, _ = r.u8()
	out.XYY, _ = r.u8()
	out.Emp, _ = r.u8()
	out.ScoutedBy, _ = r.u8()
	out.KnownBy, _ = r.u8()
	out.STyp, _ = r.u8()
	out.Typ, _ = r.u8()
	out.Tech, _ = r.u8()
	out.Eff, _ = r.u8()
	out.RevIndex, _ = r.u8()
	out.Special, _ = r.u8()
	out.Pop, _ = r.u16()
	for i := range out.Ships {
		out.Ships[i], _ = r.u16()
	}
	for i := range out.Cargo {
		out.Cargo[i], _ = r.u16()
	}
	for i := range out.Defns {
		out.Defns[i], _ = r.u16()
	}
	for i := range out.Indus {
		out.Indus[i], _ = r.u16()
	}
	out.Move, _ = r.u8()
	out.DestX, _ = r.u8()
	out.DestY, _ = r.u8()
	out.Status, _ = r.u8()
	reserved, err := r.readBytes(len(out.Reserved))
	if err != nil {
		return out, err
	}
	copy(out.Reserved[:], reserved)
	out.NextObjType, _ = r.u8()
	out.NextObjIndex, _ = r.u8()
	return out, nil
}

func (p *parser) readFleets() ([]FleetEntry, error) {
	var out []FleetEntry
	for {
		idx, err := p.u16()
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			break
		}
		if idx > maxFleets {
			return nil, fmt.Errorf("fleet index out of range: %d", idx)
		}

		rawFleet, err := p.readBytes(fleetRecordSize)
		if err != nil {
			return nil, err
		}
		rec, err := parseFleetRecord(rawFleet)
		if err != nil {
			return nil, err
		}

		noOfCommands, err := p.u16()
		if err != nil {
			return nil, err
		}
		commands := make([]CommandRecord, int(noOfCommands))
		for i := 0; i < int(noOfCommands); i++ {
			rawCmd, err := p.readBytes(commandRecordSize)
			if err != nil {
				return nil, fmt.Errorf("fleet=%d command=%d: %w", idx, i+1, err)
			}
			commands[i] = CommandRecord{
				Typ:    rawCmd[0],
				RawArg: [4]byte{rawCmd[1], rawCmd[2], rawCmd[3], rawCmd[4]},
			}
		}

		out = append(out, FleetEntry{
			Index:    idx,
			Record:   rec,
			Commands: commands,
		})
	}
	return out, nil
}

func parseFleetRecord(raw []byte) (FleetRecord, error) {
	var out FleetRecord
	r := &parser{data: raw}
	out.XYX, _ = r.u8()
	out.XYY, _ = r.u8()
	out.Emp, _ = r.u8()
	out.ScoutedBy, _ = r.u8()
	for i := range out.Ships {
		out.Ships[i], _ = r.u16()
	}
	for i := range out.Cargo {
		out.Cargo[i], _ = r.u16()
	}
	out.DestX, _ = r.u8()
	out.DestY, _ = r.u8()
	out.Status, _ = r.u8()
	out.FuelHigh, _ = r.u8()
	fuel, _ := r.u16()
	out.Fuel = int16(fuel)
	out.KnownBy, _ = r.u8()
	out.NextOrder, _ = r.u8()
	orderData, err := r.readBytes(len(out.OrderData))
	if err != nil {
		return out, err
	}
	copy(out.OrderData[:], orderData)
	out.NPEDataIndex, _ = r.u8()
	reserved, err := r.readBytes(len(out.Reserved))
	if err != nil {
		return out, err
	}
	copy(out.Reserved[:], reserved)
	out.NextObjType, _ = r.u8()
	out.NextObjIndex, _ = r.u8()
	return out, nil
}

func (p *parser) readStargates(version uint16) ([]IndexedRecord[StargateRecord], error) {
	recSize := stargateRecordSizeV15
	if version <= 14 {
		recSize = stargateRecordSizeV15 - 18 // no WLF array in older versions
	}
	return readIndexed(p, maxStargates, recSize, parseStargateRecordVersion(version))
}

func parseStargateRecordVersion(version uint16) func([]byte) (StargateRecord, error) {
	return func(raw []byte) (StargateRecord, error) {
		var out StargateRecord
		r := &parser{data: raw}
		out.XYX, _ = r.u8()
		out.XYY, _ = r.u8()
		out.Emp, _ = r.u8()
		out.ScoutedBy, _ = r.u8()
		out.KnownBy, _ = r.u8()
		out.GTyp, _ = r.u8()
		out.DestX, _ = r.u8()
		out.DestY, _ = r.u8()
		if version > 14 {
			for i := range out.WLF {
				out.WLF[i], _ = r.u16()
			}
		}
		out.NextObjType, _ = r.u8()
		out.NextObjIndex, _ = r.u8()
		return out, nil
	}
}

func (p *parser) readConstr() ([]IndexedRecord[ConstrRecord], error) {
	return readIndexed(p, maxConstrSites, constrRecordSize, parseConstrRecord)
}

func parseConstrRecord(raw []byte) (ConstrRecord, error) {
	var out ConstrRecord
	r := &parser{data: raw}
	out.XYX, _ = r.u8()
	out.XYY, _ = r.u8()
	out.Emp, _ = r.u8()
	out.ScoutedBy, _ = r.u8()
	out.KnownBy, _ = r.u8()
	out.CTyp, _ = r.u8()
	out.TimeToCompletion, _ = r.u8()
	out.NextObjType, _ = r.u8()
	out.NextObjIndex, _ = r.u8()
	return out, nil
}

func (p *parser) readMessages() ([]Message, error) {
	n, err := p.u8()
	if err != nil {
		return nil, err
	}
	out := make([]Message, int(n))
	for i := 0; i < int(n); i++ {
		recBytes, err := p.readBytes(messageRecordSize)
		if err != nil {
			return nil, err
		}
		linesN, err := p.u8()
		if err != nil {
			return nil, err
		}
		lines := make([]string, int(linesN))
		for j := 0; j < int(linesN); j++ {
			line, err := p.readPascalString(80)
			if err != nil {
				return nil, err
			}
			lines[j] = line
		}

		var raw [messageRecordSize]byte
		copy(raw[:], recBytes)
		out[i] = Message{
			Sender:      recBytes[0],
			Recipient:   recBytes[1],
			ReadBy:      recBytes[2],
			Read:        recBytes[3] != 0,
			Intercepted: recBytes[4] != 0,
			NoOfLines:   linesN,
			Lines:       lines,
			RawRecord:   raw,
		}
	}
	return out, nil
}

func (p *parser) readEmpireData() ([]EmpireData, error) {
	out := make([]EmpireData, maxEmpires)
	for i := 0; i < maxEmpires; i++ {
		recBytes, err := p.readBytes(empireDataRecordSize)
		if err != nil {
			return nil, err
		}
		nameCount, err := p.u8()
		if err != nil {
			return nil, err
		}
		names := make([]NameRecord, int(nameCount))
		for j := 0; j < int(nameCount); j++ {
			nameRaw, err := p.readBytes(nameRecordSize)
			if err != nil {
				return nil, err
			}
			names[j], err = parseNameRecord(nameRaw)
			if err != nil {
				return nil, err
			}
		}

		emp, err := parseEmpireData(recBytes)
		if err != nil {
			return nil, err
		}
		emp.Names = names
		out[i] = emp
	}
	return out, nil
}

func parseEmpireData(raw []byte) (EmpireData, error) {
	var out EmpireData
	r := &parser{data: raw}
	var err error
	var rec [empireDataRecordSize]byte
	copy(rec[:], raw)
	out.RawRecord = rec

	b, err := r.u8()
	if err != nil {
		return out, err
	}
	out.InUse = b != 0
	b, _ = r.u8()
	out.IsAPlayer = b != 0
	out.EmpireName, _ = r.readPascalString(32)
	out.Pass, _ = r.readPascalString(8)
	t, _ := r.u16()
	out.TimeLeft = int16(t)
	out.CapitalObjType, _ = r.u8()
	out.CapitalObjIndex, _ = r.u8()
	if _, err := r.readBytes(70); err != nil { // DefenseSettings raw
		return out, err
	}
	if _, err := r.readBytes(30); err != nil { // Probe array raw
		return out, err
	}
	if _, err := r.readBytes(4); err != nil { // Names ptr
		return out, err
	}
	if _, err := r.readBytes(4); err != nil { // LastName ptr
		return out, err
	}
	riv, _ := r.u16()
	_ = int16(riv) // kept in raw only
	out.TechnologyLevel, _ = r.u8()
	ts, _ := r.u32()
	out.TechnologySet = ts
	b, _ = r.u8()
	out.IsAnEmpress = b != 0
	rf, _ := r.u16()
	out.RevFactor = int16(rf)
	out.Founding, _ = r.u16()
	out.Modifiers, _ = r.u8()
	if _, err := r.readBytes(14); err != nil { // reserved
		return out, err
	}
	return out, nil
}

func parseNameRecord(raw []byte) (NameRecord, error) {
	var out NameRecord
	var rec [nameRecordSize]byte
	copy(rec[:], raw)
	out.RawRecord = rec
	r := &parser{data: raw}
	out.Name, _ = r.readPascalString(8)
	out.CoordX, _ = r.u8()
	out.CoordY, _ = r.u8()
	out.IDObjType, _ = r.u8()
	out.IDObjIndex, _ = r.u8()
	out.RawNextPtr, _ = r.u32()
	return out, nil
}

func (p *parser) readNews() ([maxEmpires][]NewsRecord, error) {
	var out [maxEmpires][]NewsRecord
	for i := 0; i < maxEmpires; i++ {
		n, err := p.u16()
		if err != nil {
			return out, err
		}
		items := make([]NewsRecord, int(n))
		for j := 0; j < int(n); j++ {
			raw, err := p.readBytes(newsRecordSize)
			if err != nil {
				return out, err
			}
			items[j], err = parseNewsRecord(raw)
			if err != nil {
				return out, err
			}
		}
		out[i] = items
	}
	return out, nil
}

func parseNewsRecord(raw []byte) (NewsRecord, error) {
	var out NewsRecord
	var rec [newsRecordSize]byte
	copy(rec[:], raw)
	out.RawRecord = rec
	r := &parser{data: raw}
	out.Headline, _ = r.u8()
	out.LocX, _ = r.u8()
	out.LocY, _ = r.u8()
	out.LocObjType, _ = r.u8()
	out.LocObjIdx, _ = r.u8()
	p1, _ := r.u16()
	p2, _ := r.u16()
	p3, _ := r.u16()
	out.Parm1 = int16(p1)
	out.Parm2 = int16(p2)
	out.Parm3 = int16(p3)
	out.RawNextPtr, _ = r.u32()
	return out, nil
}

func (p *parser) readNPEDataArray() ([maxEmpireWithAI]NPEDataRecord, error) {
	var out [maxEmpireWithAI]NPEDataRecord
	for i := 0; i < maxEmpireWithAI; i++ {
		typ, err := p.u8()
		if err != nil {
			return out, err
		}
		ptr, err := p.u32()
		if err != nil {
			return out, err
		}
		out[i] = NPEDataRecord{
			Typ:        typ,
			RawDataPtr: ptr,
		}
	}
	return out, nil
}

func (p *parser) readNPESubsections(sf *SaveFile) error {
	for empIdx := 0; empIdx < maxEmpires; empIdx++ {
		emp := sf.EmpireData[empIdx]
		if !emp.InUse || emp.IsAPlayer {
			continue
		}
		npeTyp := sf.NPEData[empIdx].Typ
		npeSize, err := sizeForNPETyp(npeTyp)
		if err != nil {
			return fmt.Errorf("empire index=%d: %w", empIdx, err)
		}
		raw, err := p.readBytes(npeSize)
		if err != nil {
			return fmt.Errorf("empire index=%d type=%d: %w", empIdx, npeTyp, err)
		}
		sf.NPESaves[uint8(empIdx)] = NPEPayload{
			EmpireIndex: uint8(empIdx),
			Typ:         npeTyp,
			Raw:         raw,
		}
	}
	return nil
}

func sizeForNPETyp(t uint8) (int, error) {
	switch t {
	case 1: // PirateNPE
		return npePirateDataSize, nil
	case 2, 3: // Kingdom1NPE..Kingdom2NPE
		return npeKingdomDataSize, nil
	case 4: // BerserkerNPE
		return npeBerserkerDataSize, nil
	case 5: // GuardianNPE
		return npeGuardianDataSize, nil
	default:
		return 0, fmt.Errorf("unsupported or inactive NPE type: %d", t)
	}
}

func readIndexed[T any](p *parser, maxIndex, recSize int, parse func([]byte) (T, error)) ([]IndexedRecord[T], error) {
	var out []IndexedRecord[T]
	for {
		idx, err := p.u16()
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			break
		}
		if int(idx) < 1 || int(idx) > maxIndex {
			return nil, fmt.Errorf("index out of range: %d max=%d", idx, maxIndex)
		}

		raw, err := p.readBytes(recSize)
		if err != nil {
			return nil, err
		}
		rec, err := parse(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, IndexedRecord[T]{Index: idx, Record: rec})
	}
	return out, nil
}

func (p *parser) u8() (uint8, error) {
	b, err := p.readBytes(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (p *parser) u16() (uint16, error) {
	b, err := p.readBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (p *parser) u32() (uint32, error) {
	b, err := p.readBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (p *parser) bool() (bool, error) {
	v, err := p.u8()
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

func (p *parser) readPascalString(maxChars int) (string, error) {
	raw, err := p.readBytes(maxChars + 1)
	if err != nil {
		return "", err
	}
	n := int(raw[0])
	if n > maxChars {
		n = maxChars
	}
	return string(raw[1 : 1+n]), nil
}

func (p *parser) readFixedString(n int) (string, error) {
	raw, err := p.readBytes(n)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (p *parser) readBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative read size: %d", n)
	}
	if p.off+n > len(p.data) {
		return nil, io.EOF
	}
	chunk := p.data[p.off : p.off+n]
	p.off += n
	return chunk, nil
}

func (p *parser) remaining() int {
	return len(p.data) - p.off
}
