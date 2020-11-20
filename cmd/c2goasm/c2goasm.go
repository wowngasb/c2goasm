/*
 * Minio Cloud Storage, (C) 2017 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"fmt"
	"github.com/wowngasb/c2goasm/asm2plan9s"
	"github.com/wowngasb/c2goasm/compiler"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	assembleFlag = flag.Bool("a", true, "Immediately invoke asm2plan9s")
	stripFlag    = flag.Bool("s", false, "Strip comments")
	compactFlag  = flag.Bool("c", false, "Compact byte codes")
	formatFlag   = flag.Bool("f", false, "Format using asmfmt")
	targetFlag   = flag.String("t", "amd64", "Target machine of input code")
)

func main() {
	flag.Parse()

	if flag.NArg() < 2 {
		fmt.Printf("error: not enough input files specified\n\n")
		fmt.Println("usage: c2goasm /path/to/c-project/build/SomeGreatCode.cpp.s SomeGreatCode_amd64.s")
		return
	}
	assemblyFile := flag.Arg(1)
	if !strings.HasSuffix(assemblyFile, ".s") {
		fmt.Printf("error: second parameter must have '.s' extension\n")
		return
	}

	goCompanion := assemblyFile[:len(assemblyFile)-2] + ".go"
	if _, err := os.Stat(goCompanion); os.IsNotExist(err) {
		fmt.Printf("error: companion '.go' file is missing for %s\n", flag.Arg(1))
		return
	}

	cfg := &compiler.Config{
		TargetFlag: *targetFlag,
	}

	fmt.Println("Processing", flag.Arg(0))
	lines, err := cfg.ReadLines(flag.Arg(0))
	if err != nil {
		log.Fatalf("readLines: %s", err)
	}

	result, err := cfg.Process(lines, goCompanion)
	if err != nil {
		fmt.Print(err)
		os.Exit(-1)
	}

	err = cfg.WriteLines(result, assemblyFile, true)
	if err != nil {
		log.Fatalf("writeLines: %s", err)
	}

	if *assembleFlag {
		fmt.Println("Invoking asm2plan9s on", assemblyFile)
		asm2plan9s.Asm2plan9s(assemblyFile)
	}

	if *stripFlag {
		cfg.StripGoasmComments(assemblyFile)
	}

	if *compactFlag {
		cfg.CompactOpcodes(assemblyFile)
	}

	if *formatFlag {
		cmd := exec.Command("asmfmt", "-w", assemblyFile)
		_, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("asmfmt: %v", err)
		}
	}
}
