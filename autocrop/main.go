package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"ktkr.us/pkg/autocrop"
)

var (
	flagFc       = flag.Float64("fc", 0.1, "cutoff frequency")
	flagThresh   = flag.Float64("d", 12, "color value d/dx considered to be page border")
	flagNSamples = flag.Int("n", 500, "number of samples to take per side")
	flagProf     = flag.Bool("prof", false, "produce a CPU profile")
)

func init() {
	log.SetFlags(0)
	flag.Parse()
}

func main() {
	if *flagProf {
		c, err := os.Create("cpu.out")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(pprof.StartCPUProfile(c))

		defer func() {
			pprof.StopCPUProfile()
			c.Close()
		}()
	}

	if flag.NArg() < 1 {
		log.Fatal("top lel")
	}

	t, err := autocrop.AnalyzeFile(flag.Arg(0), *flagThresh, *flagFc, *flagNSamples)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("convert", flag.Arg(0), t, "_"+flag.Arg(0))
	//fmt.Println("confidence", t.Confidence)
}
