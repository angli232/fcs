package fcs2_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nsbuitrago/fcs2"
)

const testFile string = "facs_validation.fcs"

func BenchmarkDecoder(b *testing.B) {
	f, err := os.Open(filepath.Join("fcs_testdata/", testFile))
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f.Seek(0, 0)
		b.StartTimer()
		_, _, err = fcs.NewDecoder(f).Decode()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetadataDecoder(b *testing.B) {
	f, err := os.Open(filepath.Join("fcs_testdata/", testFile))
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f.Seek(0, 0)
		b.StartTimer()
		_, err = fcs.NewDecoder(f).DecodeMetadata()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestDecoder_Stratedigm(t *testing.T) {
	f, err := os.Open(filepath.Join("fcs_testdata/", testFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, _, err = fcs.NewDecoder(f).Decode()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecoder_StratedigmFCS20(t *testing.T) {
	f, err := os.Open(filepath.Join("fcs_testdata/", testFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, _, err = fcs.NewDecoder(f).Decode()
	if err != nil {
		t.Fatal(err)
	}
}

func ExampleDecoder_DecodeMetadata() {
	f, err := os.Open(filepath.Join("fcs_testdata/", testFile))
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	defer f.Close()

	m, err := fcs.NewDecoder(f).DecodeMetadata()
	if err != nil {
		fmt.Printf("%s", err)
		return
	}

	fmt.Printf("NumParameters: %d\n", m.NumParameters)
	fmt.Printf("NumEvents: %d\n", m.NumEvents)
	// Output:
	// NumParameters: 10
  //NumEvents: 10925
}

func ExampleDecoder_Decode() {
	f, err := os.Open(filepath.Join("fcs_testdata/", testFile))
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	defer f.Close()

	m, data, err := fcs.NewDecoder(f).Decode()
	if err != nil {
		fmt.Printf("%s", err)
		return
	}

	np := m.NumParameters
	ne := m.NumEvents
	p := m.Parameters

	fmt.Printf("NumParameters: %d\n", np)
	fmt.Printf("NumEvents: %d\n", ne)

	fmt.Printf("Event 0:\n")
	for i := 0; i < np; i++ {
		fmt.Printf("  %s: %.4f\n", p[i].ShortName, data[i])
	}

	fmt.Printf("Event %d:\n", 1179)
	for i := 0; i < np; i++ {
		fmt.Printf("  %s: %.4f\n", p[i].ShortName, data[i+np*1179])
	}

}
