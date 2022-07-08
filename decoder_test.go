package fcs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/angli232/fcs"
)

func BenchmarkDecoder(b *testing.B) {
	f, err := os.Open(filepath.Join("../fcs_testdata", "Stratedigm.fcs"))
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
	f, err := os.Open(filepath.Join("../fcs_testdata", "Stratedigm.fcs"))
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
	f, err := os.Open(filepath.Join("../fcs_testdata", "Stratedigm.fcs"))
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
	f, err := os.Open(filepath.Join("../fcs_testdata", "Stratedigm_FCS2.0.fcs"))
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
	f, err := os.Open(filepath.Join("../fcs_testdata", "Stratedigm.fcs"))
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
	// NumParameters: 16
	// NumEvents: 6462
}

func ExampleDecoder_Decode() {
	f, err := os.Open(filepath.Join("../fcs_testdata", "Stratedigm.fcs"))
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

	// Output:
	// NumParameters: 16
	// NumEvents: 6462
	// Event 0:
	//   FSC LogH: 1360.5924
	//   FSC LinH: 1307.5256
	//   FSC LinA: 934.6008
	//   SSC LogH: 526.6931
	//   SSC LinH: 510.5591
	//   SSC LinA: 347.2900
	//   FITC(530/30) LogH: 1.0323
	//   FITC(530/30) LinH: 24.7192
	//   AmCyan(530/30) LogH: 1.1170
	//   AmCyan(530/30) LinH: 28.8391
	//   MCherry(615/30) LogH: 1.2380
	//   MCherry(615/30) LinH: 31.7383
	//   PACB(445/60) LogH: 3.5431
	//   PACB(445/60) LinH: 25.9399
	//   Width: 2187.0422
	//   Time: 0.8739
	// Event 1179:
	//   FSC LogH: 3399.1636
	//   FSC LinH: 3286.8958
	//   FSC LinA: 3827.0569
	//   SSC LogH: 1868.7251
	//   SSC LinH: 1830.9021
	//   SSC LinA: 1822.3572
	//   FITC(530/30) LogH: 377.4890
	//   FITC(530/30) LinH: 374.4507
	//   AmCyan(530/30) LogH: 3.5465
	//   AmCyan(530/30) LinH: 26.2451
	//   MCherry(615/30) LogH: 18.1540
	//   MCherry(615/30) LinH: 36.0107
	//   PACB(445/60) LogH: 1.9563
	//   PACB(445/60) LinH: 25.1770
	//   Width: 3479.7668
	//   Time: 4.0269
}
