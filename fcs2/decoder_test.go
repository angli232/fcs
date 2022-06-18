package fcs2_test

import (
	"fmt"
	"github.com/nsbuitrago/fcs2/fcs2"
	"os"
	"testing"
)

const testFile string = "../testdata/facs_validation.fcs"

func BenchmarkDecoder(b *testing.B) {
	f, err := os.Open(testFile)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f.Seek(0, 0)
		b.StartTimer()
		_, _, err = fcs2.NewDecoder(f).Decode()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetadataDecoder(b *testing.B) {
	f, err := os.Open(testFile)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f.Seek(0, 0)
		b.StartTimer()
		_, err = fcs2.NewDecoder(f).DecodeMetadata()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestDecoder_Stratedigm(t *testing.T) {
	f, err := os.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, _, err = fcs2.NewDecoder(f).Decode()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecoder_StratedigmFCS20(t *testing.T) {
	f, err := os.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, _, err = fcs2.NewDecoder(f).Decode()
	if err != nil {
		t.Fatal(err)
	}
}

func ExampleDecoder_DecodeMetadata() {
	f, err := os.Open(testFile)
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	defer f.Close()

	m, err := fcs2.NewDecoder(f).DecodeMetadata()
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
	f, err := os.Open(testFile)
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	defer f.Close()

	m, data, err := fcs2.NewDecoder(f).Decode()
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

	// NumParameters: 10
	// NumEvents: 10925
	// Event 0:
	// TIME: 0.3664
	// FSC-A: 327676.0938
	// FSC-H: 312099.1875
	// FSC-W: 255.0000
	// SSC-A: 341595.7500
	// SSC-H: 230399.6875
	// SSC-W: 255.0000
	// FL1-A: 584.0190
	// FL2-A: 16641.4102
	// FL4-A: 2582.0559

}
