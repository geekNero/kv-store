package utility

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

func ProfileCPU(cpuprofile *string) func() {
	if *cpuprofile != "" {
		log.Println("cpuprofile set to true, will be saved to: ", *cpuprofile)
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		closer := func() {
			f.Close()
			pprof.StopCPUProfile()
		}
		return closer
	}
	return nil
}

func ProfileMemory(memprofile *string) {
	if *memprofile != "" {
		log.Println("memprofile set to true, will be saved to: ", *memprofile)
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		// Lookup("allocs") creates a profile similar to go test -memprofile.
		// Alternatively, use Lookup("heap") for a profile
		// that has inuse_space as the default index.
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}

}
