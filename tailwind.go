package melt

import (
	"fmt"
	"os"
	"os/exec"
)

func (f *Furnace) runTailwind() {
	if f.productionMode {
		fmt.Println("[MELT] tailwind is not suported in production mode")
	}

	outputFile := f.tailwindOutputFile

	if outputFile == "" {
		file, err := os.CreateTemp("", "melt-tailwind-output-*.css")

		if err != nil {
			fmt.Println("[MELT] [ERROR]", err)
			return
		}

		outputFile = file.Name()
		defer os.Remove(outputFile)
	}

	cmd := exec.Command(f.tailwindExecutable, "-i", f.tailwindInputFile, "-o", outputFile, "-c", f.tailwindConfigFile, "-m")
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Println("[MELT] [TAILWIND]", string(output))
	} else {

		bytes, err := os.ReadFile(outputFile)

		if err != nil {
			fmt.Println("[MELT] [ERROR]", err)
			return
		}

		f.TailwindStyles = string(bytes)

	}
}
