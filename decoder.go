// Package fcs implements a FCS (Flow Cytometry Standard) file decoder.
package fcs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

var (
	ErrInvalidHeader   = errors.New("invalid header")
	ErrInvalidText     = errors.New("invalid TEXT segment")
	ErrKeywordNotFound = errors.New("keyword not found")
)

// FCS 3.1 Standard. 3.2.8
// Their existance will be verified when decoding the TEXT segment.
var requiredKeywords = []string{
	//	"$BEGINANALYSIS",
	//	"$BEGINDATA",
	//	"$BEGINSTEXT",
	"$BYTEORD",
	"$DATATYPE",
	//	"$ENDANALYSIS",
	//	"$ENDDATA",
	//	"$ENDSTEXT",
	"$MODE",
	"$NEXTDATA",
	"$PAR",
	"$TOT",
}

// FCS 3.1 Standard. 3.2.8
// Their existance will be verified when decoding the TEXT segment.
var requiredParameterKeywords = []string{
	"$P%dB",
	"$P%dE",
	"$P%dN",
	"$P%dR",
}

// Metadata of the parameter
type Parameter struct {
	ParameterID int

	// Required
	BitLength         int        `keyword:"$PnB"` // Number of bits reserved for parameter number n.
	AmplificationType [2]float64 `keyword:"$PnE"` // Amplification type for parameter n.
	ShortName         string     `keyword:"$PnN"` // Short name for parameter n.
	Range             int        `keyword:"$PnR"` // Range for parameter number n.

	// Optional
	Name            string   `keyword:"$PnS" json:"name,omitempty"` // Name used for parameter n.
	AmplifierGain   *float64 `keyword:"$PnG" json:"amplifiergain,omitempty"` // Amplifier gain used for acquisition of parameter n.
	DetectorType    string   `keyword:"$PnT" json:"detectortype,omitempty"` // Detector type for parameter n.
	DetectorVoltage *float64 `keyword:"$PnV" json:"detectorvoltage,omitempty"` // Detector voltage for parameter n.
	OpticalFilter   string   `keyword:"$PnF" json:"opticalfilter,omitempty"` // Name of optical filter for parameter n.

	// Non-standard parameters
	DetectorName string   `json:",omitempty"`
	Low          *float64 `keyword:"PnLO" json:",omitempty"` // Stratedigm
	High         *float64 `keyword:"PnHI" json:",omitempty"` // Stratedigm
}

// Metadata
type Metadata struct {
	FCSVersion string

	// Required parameters (FCS 3.1. Section 3.2.18)
	BeginSupplementalText int    `keyword:"$BEGINSTEXT"`    // Byte-offset to the beginning of a supplemental TEXT segment.
	EndSupplementalText   int    `keyword:"$ENDSTEXT"`      // Byte-offset to the last byte of a supplemental TEXT segment.
	BeginData             int    `keyword:"$BEGINDATA"`     // Byte-offset to the beginning of the DATA segment.
	EndData               int    `keyword:"$ENDDATA"`       // Byte-offset to the last byte of the DATA segment.
	BeginAnalysis         int    `keyword:"$BEGINANALYSIS"` // Byte-offset to the beginning of the ANALYSIS segment.
	EndAnalysis           int    `keyword:"$ENDANALYSIS"`   // Byte-offset to the last byte of the ANALYSIS segment.
	NextData              int    `keyword:"$NEXTDATA"`      // Byte offset to next data set in the file.
	ByteOrder             string `keyword:"$BYTEORD"`       // Byte order for data acquisition computer.
	DataType              string `keyword:"$DATATYPE"`      // Type of data in DATA segment (ASCII, integer, floating point).
	Mode                  string `keyword:"$MODE"`          // Data mode (list mode - preferred, histogram - deprecated).
	NumEvents             int    `keyword:"$TOT"`           // Total number of events in the data set.
	NumParameters         int    `keyword:"$PAR"`           // Number of parameters in an event.
  Parameters            []Parameter `json:"parameters"`

	// (Some) Optional parameters (FCS 3.1. Section 3.2.19)
	FileName            string    `keyword:"$FIL" json:"filename,omitempty"`                              // Name of the data file containing the data set.
	Operator            string    `keyword:"$OP" json:"operator,omitempty"`                               // Name of flow cytometry operator.
	PlateID             string    `keyword:"$PLATEID,PLATE_ID,PLATE ID" json:"plateid,omitempty"`        // Plate identifier. Stratedigm(PLATE_ID, not globally unique). LSRII(PLATE ID)
	PlateName           string    `keyword:"$PLATENAME,PLATE NAME,SAMPLE_NAME" json:"platename,omitempty"` // Plate name. LSRII(PLATE NAME). Stratedigm(SAMPLE_NAME)
	WellID              string    `keyword:"$WELLID,WELL ID,WELL_ID" json:"wellid,omitempty"`           // Well identifier (e.g. A07). LSRII(WELL ID) Stratedigm(WELL_ID)
	Date                time.Time `keyword:"$DATE" json:"date,omitempty"`                             // Date of data set acquisition.
	BeginTime           time.Time `keyword:"$BTIM" json:"begintime,omitempty"`                             // Clock time at beginning of data acquisition.
	EndTime             time.Time `keyword:"$ETIM" json:"endtime,omitempty"`                             // Clock time at end of data acquisition.
	ComputerSystem      string    `keyword:"$SYS" json:"computersystem,omitempty"`                              // Type of computer and its operating system.
	CytometerType       string    `keyword:"$CYT" json:"computertype,omitempty"`                              // Type of flow cytometer.
	CytometerSN         string    `keyword:"$CYTSN,CYTNUM" json:"cytometersn,omitempty"`                     // Flow cytometer serial number. LSRII(CYTNUM)
	TimeStep            *float64  `keyword:"$TIMESTEP" json:"timestep,omitempty"`                         // Time step for time parameter.
	Volume              *float64  `keyword:"$VOL" json:"volume,omitempty"`                              // Volume of sample run during data acquisition (in nanoliters).
	SpecimenSource      string    `keyword:"$SRC" json:"specimensource,omitempty"`                              // Source of the specimen (patient name, cell types)
	SpecimenLabel       string    `keyword:"$SMNO" json:"specimenlabel,omitempty"`                             // Specimen (e.g., tube) label.
	SpecimenType        string    `keyword:"$CELLS" json:"specimentype,omitempty"`                            // Type of cells or other objects measured.
	NumLostEvent        int       `keyword:"$LOST" json:"numlostevent,omitempty"`                             // Number of events lost due to computer busy.
	NumAbortedEvent     int       `keyword:"$ABRT" json:"numabortedevent,omitempty"`                             // Events lost due to data acquisition electronic coincidence.
	Originality         string    `keyword:"$ORIGINALITY" json:"originality,omitempty"`                      // Information whether the FCS data set has been modified (any part of it) or is original as acquired by the instrument.
	Institution         string    `keyword:"$INST" json:"institution,omitempty"`                             // Institution at which data was acquired.
	Comment             string    `keyword:"$COM" json:"comment,omitempty"`                              // Comment.
	ExperimentInitiator string    `keyword:"$EXP" json:"experimentinitiator,omitempty"`                              // The name of the person initiating the experiment.

	// Non-standard parameters
	Software       string   `keyword:"SOFTWARE,CREATOR" json:",omitempty"`                // Stratedigm(SOFTWARE), LSRII(CREATOR)
	ExperimentName string   `keyword:"EXPERIMENT_NAME,EXPERIMENT NAME" json:",omitempty"` // Stratedigm(EXPERIMENT_NAME), LSRII(EXPERIMENT NAME)
	ExperimentID   string   `keyword:"SF_EXPERIMENT_UID" json:",omitempty"`               // Stratedigm
	TubeName       string   `keyword:"TUBE_NAME,TUBE NAME" json:",omitempty"`             // Stratedigm(TUBE_NAME), LSRII(TUBE NAME)
	FlowRate       *float64 `keyword:"#FLOWRATE" json:",omitempty"`                       // Attune

	// Raw data
	delimiter byte
	keywords  []string
	kv        map[string]string
}

// Keywords returns all keywords following the order in the file.
func (m *Metadata) Keywords() []string {
	return m.keywords
}

// Raw returns the key-value map of all metadata from the TEXT segment of the file.
func (m *Metadata) Raw() map[string]string {
	return m.kv
}

type header struct {
	FCSVersion    string
	TextStart     int // offset to first byte of TEXT segment
	TextEnd       int // offset to last byte of TEXT segment
	DataStart     int // offset to first byte of DATA segment
	DataEnd       int // offset to last byte of DATA segment
	AnalysisStart int // offset to first byte of ANALYSIS segment
	AnalysisEnd   int // offset to last byte of ANALYSIS segment
}

type Decoder struct {
	r io.Reader

	header   *header
	metadata *Metadata
}

// NewDecoder returns a decoder for the FCS format (FCS 2.0, 3.0, 3.1).
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r: r,
	}
}

// DecodeMetadata decodes and returns only the metadata sections.
func (dec *Decoder) DecodeMetadata() (*Metadata, error) {
	if dec.metadata != nil {
		return dec.metadata, nil
	}

	// Read header
	h, n, err := decodeHeader(dec.r)
	if err != nil {
		return nil, err
	}
	dec.header = h

	// Advance to the beginning of TEXT segment
	_, err = io.CopyN(ioutil.Discard, dec.r, int64(h.TextStart-n))
	if err != nil {
		return nil, err
	}

	// Read TEXT segment
	textSegmentLength := h.TextEnd - h.TextStart + 1
	m, err := decodeText(io.LimitReader(dec.r, int64(textSegmentLength)))
	if err != nil {
		return m, err
	}

	// Fill FCS version from header
	m.FCSVersion = h.FCSVersion

	return m, nil
}

// Decode decodes and returns both the metadata and the data.
// The data is []float64 with the length of (m.NumParameters x m.NumEvents).
// An event is represented as a vector of the n parameters [p1, p2, p3, ... pn].
// The data array is in the form of [p1, p2, p3, ..., pn, p1, p2, p3, ... pn, ...].
func (dec *Decoder) Decode() (m *Metadata, data []float64, err error) {
	m, err = dec.DecodeMetadata()
	if err != nil {
		return
	}

	// Advance to the beginning of DATA segment
	_, err = io.CopyN(ioutil.Discard, dec.r, int64(dec.header.DataStart-dec.header.TextEnd-1))
	if err != nil {
		return nil, nil, err
	}

	dataSegmentLength := dec.header.DataEnd - dec.header.DataStart + 1
	data, err = decodeData(io.LimitReader(dec.r, int64(dataSegmentLength)), m)
	return
}

func decodeHeader(r io.Reader) (h *header, n int, err error) {
	buf := make([]byte, 0, 8)

	h = &header{}

	// FCS Version: 00-05
	buf = buf[:6]
	nr, err := r.Read(buf)
	n += nr
	if nr != len(buf) || err == io.EOF {
		return nil, n, ErrInvalidHeader
	}
	if err != nil {
		return nil, n, err
	}
	h.FCSVersion = string(buf)
	switch h.FCSVersion {
	case "FCS2.0", "FCS3.0", "FCS3.1":
		break
	default:
		return nil, n, ErrInvalidHeader
	}

	// Spaces
	buf = buf[:4]
	nr, err = r.Read(buf)
	n += nr
	if nr != len(buf) || err == io.EOF {
		return nil, n, ErrInvalidHeader
	}
	if err != nil {
		return nil, n, err
	}
	for _, char := range buf {
		if char != ' ' {
			return nil, n, ErrInvalidHeader
		}
	}

	// Offsets of TEXT, DATA, ANALYSIS
	var offsets [6]int
	buf = buf[:8]

	for i := 0; i < 6; i++ {
		nr, err = r.Read(buf)
		n += nr
		if nr != len(buf) || err == io.EOF {
			return nil, n, ErrInvalidHeader
		}
		if err != nil {
			return nil, n, err
		}
		offsets[i], err = strconv.Atoi(string(bytes.TrimSpace(buf)))
		if err != nil {
			return nil, n, ErrInvalidHeader
		}
	}

	h.TextStart = offsets[0]
	h.TextEnd = offsets[1]
	h.DataStart = offsets[2]
	h.DataEnd = offsets[3]
	h.AnalysisStart = offsets[4]
	h.AnalysisEnd = offsets[5]

	return h, n, nil
}

// FCS 3.1 Standard. 3.2 TEXT Segment
func decodeText(r io.Reader) (m *Metadata, err error) {
	// 3.2.5: The first character in the primary TEXT segment is the ASCII delimiter character.
	b := bufio.NewReader(r)
	delimiter, err := b.ReadByte()
	if err != nil {
		return
	}

	m = &Metadata{
		delimiter: delimiter,
		keywords:  make([]string, 0),
		kv:        make(map[string]string),
	}

	// Read all the keyword-value pairs into map[string]string m.kv
	// while keeping the order of the keywords in m.keywords
	for {
		keyword, err := b.ReadString(delimiter)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Read the value, which may uses the delimiter to escape itself.
		value := ""
		for {
			str, err := b.ReadString(delimiter)
			if err != nil {
				if err == io.EOF {
					return nil, ErrInvalidText
				}
				return nil, err
			}
			value += str

			nextChar, err := b.ReadByte()
			if err != nil {
				if err == io.EOF {
					// This happens if we are at the end of the TEXT segment.
					break
				}
				return nil, err
			}
			if nextChar != delimiter {
				// If the delimiter is not for escaping,
				// return it to the buffer and stop the value reading loop.
				err = b.UnreadByte()
				if err != nil {
					return nil, err
				}
				break
			}
		}

		// Check and remove the delimiter
		// If anything is wrong here, it is pretty likely that the file (or this parser) is very wrong.
		// So the metadata is not returned, as it will not be usable anyways.
		if keyword[len(keyword)-1] != delimiter || value[len(value)-1] != delimiter {
			return nil, ErrInvalidText
		}
		keyword = keyword[0 : len(keyword)-1]
		value = value[0 : len(value)-1]

		// Keywords are case-insensitive. The convention is upper case.
		// So convert all the keywords to upper case for easier looking up.
		strings.ToUpper(keyword)

		m.keywords = append(m.keywords, keyword)
		m.kv[keyword] = strings.TrimSpace(value) // Additional spaces are seen in LSRII's fcs files.
	}

	// Check we have read the entire TEXT segment
	// Don't return the partially parsed metadata if anything is wrong,
	// since parsed data is very likely to be wrong.
	n, _ := io.Copy(ioutil.Discard, r)
	if n > 0 {
		return nil, fmt.Errorf("%d bytes left after decoding TEXT segment. The file is corrupted or unsupported", n)
	}

	// Parse into the fields of the struct
	metadataValue := reflect.ValueOf(m).Elem()
	for i := 0; i < metadataValue.NumField(); i++ {
		keywords := metadataValue.Type().Field(i).Tag.Get("keyword")
		if keywords == "" {
			continue
		}
		var value string
		var ok bool
		for _, keyword := range strings.Split(keywords, ",") {
			value, ok = m.kv[keyword]
			if ok {
				break
			}
		}
		if value != "" {
			err = scanValueToStructField(value, metadataValue.Field(i))
			if err != nil {
				return m, err
			}
		}

	}

	// Parse the metadata of parameters
	m.Parameters = make([]Parameter, 0, m.NumParameters)
	for i := 1; i <= m.NumParameters; i++ {
		p := &Parameter{
			ParameterID: i,
		}

		paramValue := reflect.ValueOf(p).Elem()

		for j := 0; j < paramValue.NumField(); j++ {
			keyword := paramValue.Type().Field(j).Tag.Get("keyword")
			if keyword == "" {
				continue
			}
			if strings.Index(keyword, "n") < 0 {
				// panic here, since the problem will appear when testing the package with any fcs file
				panic("a keyword tag in struct Parameter does not contain 'n' as the placeholder for parameter number")
			}
			keyword = strings.Replace(keyword, "n", strconv.Itoa(i), 1)
			value, ok := m.kv[keyword]
			if !ok {
				continue
			}

			err = scanValueToStructField(value, paramValue.Field(j))
			if err != nil {
				return m, err
			}
		}

		m.Parameters = append(m.Parameters, *p)
	}

	// Special case: change the representation of byte order to make it more readable,
	// so that this package can be used without refering to the FCS format specification.
	value, ok := m.kv["$BYTEORD"]
	if !ok {
		return m, fmt.Errorf("required parameter $BYTEORD not found")
	}
	switch value {
	case "1,2,3,4":
		m.ByteOrder = "LittleEndian"
	case "4,3,2,1":
		m.ByteOrder = "BigEndian"
	default:
		return m, fmt.Errorf("unknown byte order %s", value)
	}

	// Special case: add date to begin and end time
	if !m.Date.IsZero() {
		if !m.BeginTime.IsZero() {
			m.BeginTime = time.Date(m.Date.Year(), m.Date.Month(), m.Date.Day(), m.BeginTime.Hour(), m.BeginTime.Minute(), m.BeginTime.Second(), m.BeginTime.Nanosecond(), m.Date.Location())
		}
		if !m.EndTime.IsZero() {
			m.EndTime = time.Date(m.Date.Year(), m.Date.Month(), m.Date.Day(), m.EndTime.Hour(), m.EndTime.Minute(), m.EndTime.Second(), m.EndTime.Nanosecond(), m.Date.Location())
		}
		if m.EndTime.Before(m.BeginTime) {
			// In this case, it is a good assumption that the end time is the next day.
			m.EndTime = m.EndTime.AddDate(0, 0, 1)
		}
	}

	// Validate the existance of required keywords
	// The metadata will still be returned to the user,
	// but obviously we will not be able to decode the data.
	for _, keyword := range requiredKeywords {
		_, ok := m.kv[keyword]
		if !ok {
			return m, fmt.Errorf("missing required keyword %s", keyword)
		}
	}
	for i := 1; i <= m.NumParameters; i++ {
		for _, keywordFmt := range requiredParameterKeywords {
			keyword := fmt.Sprintf(keywordFmt, i)
			_, ok := m.kv[keyword]
			if !ok {
				return m, fmt.Errorf("missing required keyword %s", keyword)
			}
		}
	}

	return m, nil
}

// scanValueToStructField interprete and store the value string according to the type of the struct field.
func scanValueToStructField(value string, field reflect.Value) error {
	switch field.Type() {
	case reflect.TypeOf(""):
		field.SetString(value)
	case reflect.TypeOf(int(0)), reflect.PtrTo(reflect.TypeOf(int(0))):
		if value == "NA" {
			// In Attune's fcs file, time parameter has $P1V=NA
			return nil
		}
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("cannot parse '%s' as int", value)
		}
		if field.Type() == reflect.TypeOf(int(0)) {
			field.SetInt(int64(intValue))
		} else {
			field.Set(reflect.ValueOf(&intValue))
		}
	case reflect.TypeOf(float64(0)), reflect.PtrTo(reflect.TypeOf(float64(0))):
		if value == "NA" {
			// In Attune's fcs file, time parameter has $P1V=NA
			return nil
		}
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("cannot parse '%s' as float64", value)
		}
		if field.Type() == reflect.TypeOf(float64(0)) {
			field.SetFloat(floatValue)
		} else {
			field.Set(reflect.ValueOf(&floatValue))
		}
	case reflect.TypeOf([2]float64{0, 0}):
		// This is basically only for amplification type
		strList := strings.Split(value, ",")
		if len(strList) != 2 {
			return fmt.Errorf("cannot parse '%s' as [2]float64", value)

		}
		f1, err := strconv.ParseFloat(strings.TrimSpace(strList[0]), 64)
		if err != nil {
			return fmt.Errorf("cannot parse '%s' as [2]float64", value)
		}
		f2, err := strconv.ParseFloat(strings.TrimSpace(strList[1]), 64)
		if err != nil {
			return fmt.Errorf("cannot parse %s as [2]float64", value)
		}
		field.Set(reflect.ValueOf([2]float64{f1, f2}))
	case reflect.TypeOf(time.Time{}):
		// The field may be a date
		t, err := time.ParseInLocation("02-Jan-2006", value, time.UTC)
		if err == nil {
			field.Set(reflect.ValueOf(t))
			return nil
		}
		// The field may be a time in the form of hh:mm:ss[.cc] (FCS 3.1 standard)
		t, err = time.ParseInLocation("15:04:05", value, time.UTC)
		if err == nil {
			field.Set(reflect.ValueOf(t))
			return nil
		}
		t, err = time.ParseInLocation("15:04:05.00", value, time.UTC)
		if err == nil {
			field.Set(reflect.ValueOf(t))
			return nil
		}
		// Another one: $LAST_MODIFIED/dd-mmm-yyyy hh:mm:ss[.cc]/
		t, err = time.ParseInLocation("02-Jan-2006 15:04:05", value, time.UTC)
		if err == nil {
			field.Set(reflect.ValueOf(t))
			return nil
		}
		t, err = time.ParseInLocation("02-Jan-2006 15:04:05.00", value, time.UTC)
		if err == nil {
			field.Set(reflect.ValueOf(t))
			return nil
		}
		// The field for $BTIM, $ETIM may be a time in the form of hh:mm:ss:tt (FCS 3.0 standard)
		// In which tt is in 1/60 of a second unit.
		var timeFormat = regexp.MustCompile(`^\d{1,2}:\d{1,2}:\d{1,2}:\d{1,2}$`)
		if timeFormat.MatchString(value) {
			strs := strings.Split(value, ":")
			hh, err := strconv.Atoi(strs[0])
			if err != nil {
				panic(err)
			}
			mm, err := strconv.Atoi(strs[1])
			if err != nil {
				panic(err)
			}
			ss, err := strconv.Atoi(strs[2])
			if err != nil {
				panic(err)
			}
			tt, err := strconv.Atoi(strs[3])
			if err != nil {
				panic(err)
			}
			t = time.Date(1, 1, 1, hh, mm, ss, int(float64(tt)/60*1e9), time.UTC)
			field.Set(reflect.ValueOf(t))
			return nil
		}
		return fmt.Errorf("cannot parse %s as time.Time", value)
	default:
		// This should not happen if this parser is implemented correctly.
		// panic here, since the problem will appear when testing the package with any fcs file
		panic(fmt.Sprintf("not parsed, unknown type: %v\n", field.Type()))
	}
	return nil
}

// FCS 3.1 Standard. 3.3 DATA Segment
func decodeData(r io.Reader, m *Metadata) (data []float64, err error) {
	if m.kv["$MODE"] != "L" {
		return nil, fmt.Errorf("only list mode is supported as data mode")
	}
	defer func() {
		// Check we have read the entire DATA segment
		n, err := io.Copy(ioutil.Discard, r)
		if n > 0 {
			err = fmt.Errorf("%d bytes left after decoding DATA segment", n)
			return
		}
		if err == nil && m.NextData != 0 {
			err = fmt.Errorf("this file contains multiple dataset, which is not supported by this parser, only the first dataset is returned")
			return
		}
	}()

	np := m.NumParameters
	ne := m.NumEvents
	data = make([]float64, np*ne)

	// Shortcut for empty data record
	if len(data) == 0 {
		return data, nil
	}

	var byteOrder binary.ByteOrder
	switch m.ByteOrder {
	case "LittleEndian":
		byteOrder = binary.LittleEndian
	case "BigEndian":
		byteOrder = binary.BigEndian
	default:
		panic(fmt.Sprintf("metadata parser gives unknown byte order %s", m.ByteOrder))
	}

	switch m.kv["$DATATYPE"] {
	case "A":
		return nil, fmt.Errorf("ASCII data type is deprecated in FCS 3.1 and not implemented by this decoder")
	case "D":
		err = binary.Read(r, byteOrder, &data)
		return data, err
	case "F":
		float32Data := make([]float32, np*ne)
		err = binary.Read(r, byteOrder, &float32Data)
		if err != nil {
			return nil, err
		}
		for i := 0; i < np*ne; i++ {
			data[i] = float64(float32Data[i])
		}
		return data, err
	case "I":
		err := decodeIntData(r, m, &data)
		return data, err
	}
	return nil, fmt.Errorf("unknown data type: %s", m.kv["$DATATYPE"])
}

func decodeIntData(r io.Reader, m *Metadata, data *[]float64) error {
	np := m.NumParameters
	ne := m.NumEvents

	if m.ByteOrder != "LittleEndian" {
		return fmt.Errorf("currently only little endian is implemented")
	}

	// Calculate the length of an event and each parameter
	paramBits := make([]int, np)
	paramBytes := make([]int, np)
	eventBytes := 0
	for i := 0; i < np; i++ {
		n := m.Parameters[i].BitLength
		switch n {
		case 8, 16, 32, 64:
			paramBits[i] = n
			paramBytes[i] = n / 8
			eventBytes += n / 8
		default:
			return fmt.Errorf("%d-bit data is not yet supported", paramBits)
		}
	}

	// Read all the data into a []byte
	buf := make([]byte, ne*eventBytes)
	nr, err := r.Read(buf)
	if err != nil {
		if err != io.EOF {
			return err
		}
	}
	if nr != ne*eventBytes {
		return fmt.Errorf("not enough bytes read")
	}

	if len(buf) == 0 {
		// Otherwise &buf[0] may panic due to index out of range
		return nil
	}

	// Convert to float64
	// Pointer arithmetic is used for the speed.
	// binary.Read + relection will take more than twice the time.
	paramOffset := 0
	bufOffset := uintptr(unsafe.Pointer(&buf[0]))
	for i := 0; i < np; i++ {
		bPtr := bufOffset
		nData := paramOffset
		switch paramBits[i] {
		case 8:
			for j := 0; j < ne; j++ {
				(*data)[nData] = float64(*(*uint8)(unsafe.Pointer(bPtr)))
				nData += np
				bPtr += uintptr(eventBytes)
			}
		case 16:
			for j := 0; j < ne; j++ {
				(*data)[nData] = float64(*(*uint16)(unsafe.Pointer(bPtr)))
				nData += np
				bPtr += uintptr(eventBytes)
			}
		case 32:
			for j := 0; j < ne; j++ {
				(*data)[nData] = float64(*(*uint32)(unsafe.Pointer(bPtr)))
				nData += np
				bPtr += uintptr(eventBytes)
			}
		case 64:
			for j := 0; j < ne; j++ {
				(*data)[nData] = float64(*(*uint64)(unsafe.Pointer(bPtr)))
				nData += np
				bPtr += uintptr(eventBytes)
			}
		default:
			panic(fmt.Sprintf("bit size of %d should not exist in this loop", paramBits[i]))
		}
		paramOffset++
		bufOffset += uintptr(paramBytes[i])
	}

	err = applyTransform(data, m)
	return err
}

// Apply linear antilog transform
func applyTransform(data *[]float64, m *Metadata) error {
	np := m.NumParameters
	ne := m.NumEvents

	for i, p := range m.Parameters {
		f1 := p.AmplificationType[0]
		f2 := p.AmplificationType[1]
		if f1 == 0 && f2 == 0 {
			// Linear transform
			if p.AmplifierGain == nil {
				continue
			}
			gain := *p.AmplifierGain
			for j := i; j < np*ne; j += np {
				(*data)[j] = (*data)[j] / gain
			}
		} else {
			// FCS 3.1 Standard. 3.2.20. Page 22.
			// The standard says f1 > 0, f2 = 0 is not valid.
			// But if it is found, handle it as $PnE/f1,1/.
			if f2 == 0 {
				f2 = 1
			}
			// Convert from log to linear
			r := float64(p.Range)
			for j := i; j < np*ne; j += np {
				// TODO: This is slow. Maybe use a lookup table to make it faster.
				(*data)[j] = math.Pow(10, f1*(*data)[j]/r) * f2
			}
		}
	}

	return nil
}
