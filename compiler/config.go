package compiler

import (
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
)

type Config struct {
	TargetFlag   string
}

func (cfg *Config) ReadLines(path string) ([]string, error) {
	return readLines(path)
}

func (cfg *Config) WriteLines(lines []string, path string, header bool) error {
	return writeLines(lines, path, header)
}

func (cfg *Config) Process(assembly []string, goCompanionFile string) ([]string, error) {

	// Split out the assembly source into subroutines
	subroutines := segmentSource(assembly)
	tables := cfg.segmentConstTables(assembly)

	var result []string

	// Iterate over all subroutines
	for isubroutine, sub := range subroutines {

		golangArgs, golangReturns := parseCompanionFile(goCompanionFile, sub.name)
		stackArgs := argumentsOnStack(sub.body)
		if len(golangArgs) > 6 && len(golangArgs)-6 < stackArgs.Number {
			panic(fmt.Sprintf("Found too few arguments on stack (%d) but needed %d", len(golangArgs)-6, stackArgs.Number))
		}

		// Check for constants table
		if table := getCorrespondingTable(sub.body, tables); table.isPresent() {

			// Output constants table
			result = append(result, strings.Split(table.Constants, "\n")...)
			result = append(result, "") // append empty line

			sub.table = table
		}

		// Create object to get offsets for stack pointer
		stack := NewStack(sub.epilogue, len(golangArgs), scanBodyForCalls(sub))

		// Write header for subroutine in go assembly
		result = append(result, writeGoasmPrologue(sub, stack, golangArgs, golangReturns)...)

		// Write body of code
		assembly, err := writeGoasmBody(sub, stack, stackArgs, golangArgs, golangReturns)
		if err != nil {
			panic(fmt.Sprintf("writeGoasmBody: %v", err))
		}
		result = append(result, assembly...)

		if isubroutine < len(subroutines)-1 {
			// Empty lines before next subroutine
			result = append(result, "\n", "\n")
		}
	}

	return result, nil
}

func (cfg *Config) StripGoasmComments(file string) {
	lines, err := readLines(file)
	if err != nil {
		log.Fatalf("ReadLines: %s", err)
	}

	for i, l := range lines {
		if strings.Contains(l, "LONG") || strings.Contains(l, "WORD") || strings.Contains(l, "BYTE") {
			opcode := strings.TrimSpace(strings.SplitN(l, "//", 2)[0])
			lines[i] = strings.SplitN(l, opcode, 2)[0] + opcode
		}
	}

	err = writeLines(lines, file, false)
	if err != nil {
		log.Fatalf("writeLines: %s", err)
	}
}

func (cfg *Config) CompactOpcodes(file string) {
	lines, err := readLines(file)
	if err != nil {
		log.Fatalf("ReadLines: %s", err)
	}

	var result []string

	opcodes := make([]byte, 0, 1000)

	hexMatch := regexp.MustCompile(`(\$0x[0-9a-f]+)`)

	for _, l := range lines {
		if strings.Contains(l, "LONG") || strings.Contains(l, "WORD") || strings.Contains(l, "BYTE") {
			match := hexMatch.FindAllStringSubmatch(l, -1)
			for _, m := range match {
				dst := make([]byte, hex.DecodedLen(len(m[0][3:])))
				_, err := hex.Decode(dst, []byte(m[0][3:]))
				if err != nil {
					log.Fatal(err)
				}
				for i := len(dst) - 1; i >= 0; i -= 1 { // append starting with lowest byte first
					opcodes = append(opcodes, dst[i:i+1]...)
				}
			}
		} else {

			if len(opcodes) != 0 {
				result = append(result, compactArray(opcodes)...)
				opcodes = opcodes[:0]
			}

			result = append(result, l)
		}
	}

	err = writeLines(result, file, false)
	if err != nil {
		log.Fatalf("writeLines: %s", err)
	}
}

/*  private func */

func (cfg *Config) segmentConstTables(lines []string) []Table {

	consts := []Const{}

	globals := splitOnGlobals(lines)

	if len(globals) == 0 {
		return []Table{}
	}

	splitBegin := 0
	for _, global := range globals {
		start := getFirstLabelConstants(lines[splitBegin:global.dotGlobalLine])
		if start != -1 {
			// Add set of lines when a constant table has been found
			consts = append(consts, Const{name: fmt.Sprintf("LCDATA%d", len(consts)+1), start: splitBegin + start, end: global.dotGlobalLine})
		}
		splitBegin = global.dotGlobalLine + 1
	}

	tables := []Table{}

	for _, c := range consts {

		tables = append(tables, defineTable(lines[c.start:c.end], c.name, cfg.TargetFlag))
	}

	return tables
}
