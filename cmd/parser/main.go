package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// LineType represents the type of content on a line
type LineType int

const (
	LineTypeOther LineType = iota
	LineTypeQuestion
	LineTypeAnswer
	LineTypeAnswerContinuation
)

// ProcessedLine holds a line with its cleaned version and type
type ProcessedLine struct {
	Cleaned string
	Type    LineType
}

func main() {
	basePath := "/home/chaschel/Documents/ibm/ai/bchat/docs/templates/examples/inc"
	inputPath := basePath + "/funda_beliefs.txt"
	qaOutputPath := basePath + "/QA.txt"
	cleanOutputPath := basePath + "/funda_beliefs_clean.txt"
	nonQAOutputPath := basePath + "/NON_QA.txt"
	contextOutputPath := basePath + "/QA_IN_CONTEXT.txt"

	// Read input file
	content, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")

	var processedLines []ProcessedLine
	var qaPairs []string

	var currentQ, currentA string
	inAnswer := false

	// First pass: clean lines and classify them
	for _, line := range lines {
		cleanedLine := cleanLine(line)
		trimmed := strings.TrimSpace(cleanedLine)

		pl := ProcessedLine{Cleaned: cleanedLine, Type: LineTypeOther}

		if strings.HasPrefix(trimmed, "Q.") {
			// Save previous Q&A pair if exists
			if currentQ != "" && currentA != "" {
				qaPairs = append(qaPairs, fmt.Sprintf("%s\n%s\n", currentQ, currentA))
			}
			currentQ = trimmed
			currentA = ""
			inAnswer = false
			pl.Type = LineTypeQuestion
		} else if strings.HasPrefix(trimmed, "A.") {
			currentA = trimmed
			inAnswer = true
			pl.Type = LineTypeAnswer
		} else if inAnswer && trimmed != "" && !strings.HasPrefix(trimmed, "Q.") {
			if !isNewSection(trimmed) {
				currentA += " " + trimmed
				pl.Type = LineTypeAnswerContinuation
			} else {
				// New section breaks the answer
				inAnswer = false
			}
		} else if inAnswer && trimmed == "" {
			// Empty line ends answer continuation
			inAnswer = false
		}

		processedLines = append(processedLines, pl)
	}

	// Don't forget last Q&A pair
	if currentQ != "" && currentA != "" {
		qaPairs = append(qaPairs, fmt.Sprintf("%s\n%s\n", currentQ, currentA))
	}

	// Write QA.txt
	if err := writeQAFile(qaOutputPath, qaPairs); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing QA file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Extracted %d Q&A pairs to %s\n", len(qaPairs), qaOutputPath)

	// Write funda_beliefs_clean.txt (all cleaned lines)
	if err := writeCleanFile(cleanOutputPath, processedLines); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing clean file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote cleaned document to %s\n", cleanOutputPath)

	// Write NON_QA.txt (only non-Q&A lines)
	if err := writeNonQAFile(nonQAOutputPath, processedLines); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing non-QA file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote non-Q&A content to %s\n", nonQAOutputPath)

	// Write QA_IN_CONTEXT.txt (full doc with Q&A marked)
	if err := writeContextFile(contextOutputPath, processedLines); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing context file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote Q&A in context to %s\n", contextOutputPath)
}

func writeQAFile(path string, qaPairs []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, qa := range qaPairs {
		f.WriteString(qa + "\n")
	}
	return nil
}

func writeCleanFile(path string, lines []ProcessedLine) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, pl := range lines {
		f.WriteString(pl.Cleaned + "\n")
	}
	return nil
}

func writeNonQAFile(path string, lines []ProcessedLine) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, pl := range lines {
		if pl.Type == LineTypeOther {
			f.WriteString(pl.Cleaned + "\n")
		}
	}
	return nil
}

func writeContextFile(path string, lines []ProcessedLine) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	inQABlock := false

	for _, pl := range lines {
		isQALine := pl.Type == LineTypeQuestion || pl.Type == LineTypeAnswer || pl.Type == LineTypeAnswerContinuation

		if isQALine && !inQABlock {
			f.WriteString("--- Q&A START ---\n")
			inQABlock = true
		} else if !isQALine && inQABlock {
			f.WriteString("--- Q&A END ---\n")
			inQABlock = false
		}

		f.WriteString(pl.Cleaned + "\n")
	}

	// Close any open Q&A block at end of file
	if inQABlock {
		f.WriteString("--- Q&A END ---\n")
	}

	return nil
}

// cleanLine removes corrupted OCR tokens from a line
func cleanLine(line string) string {
	words := strings.Fields(line)
	var cleaned []string

	for _, word := range words {
		if isValidToken(word) {
			cleaned = append(cleaned, word)
		}
	}

	return strings.Join(cleaned, " ")
}

// isValidToken checks if a token should be preserved
func isValidToken(token string) bool {
	if len(token) < 2 {
		return true // Single chars are OK
	}

	// Preserve Bible verse references (e.g., "Heb.", "Jn.", "Mt.", "II", "Tim.")
	if isBibleReference(token) {
		return true
	}

	// Preserve Roman numerals
	if isRomanNumeral(token) {
		return true
	}

	// Preserve numbers (page numbers, years, verse numbers)
	if isNumeric(token) {
		return true
	}

	// Preserve punctuation-heavy strings that look normal (e.g., "...")
	if isPunctuation(token) {
		return true
	}

	// Check for corrupted patterns
	if hasCorruptedPattern(token) {
		return false
	}

	return true
}

// hasCorruptedPattern detects OCR garbage
func hasCorruptedPattern(token string) bool {
	// Check for non-printable or unusual Unicode characters
	for _, r := range token {
		// Allow basic ASCII letters, numbers, common punctuation
		if r > 127 {
			// Check for common non-ASCII that's OK (smart quotes, etc.)
			if r == '\u2018' || r == '\u2019' || r == '\u201c' || r == '\u201d' || r == '\u2013' || r == '\u2014' {
				continue
			}
			// Other non-ASCII in the middle of a word = likely corruption
			return true
		}
		// Check for control characters
		if unicode.IsControl(r) && r != '\t' && r != '\n' {
			return true
		}
	}

	// Pattern: random mix of special chars (e.g., "/o\1111", "W".�l1J.")
	specialCharPattern := regexp.MustCompile(`[\\\/\|]+\d+|[\x00-\x1f]|\{.*\}|<.*>`)
	if specialCharPattern.MatchString(token) {
		return true
	}

	// Pattern: excessive underscores or weird symbol combos
	weirdPattern := regexp.MustCompile(`_{2,}|[_\-]{3,}|[^\w\s]{4,}`)
	if weirdPattern.MatchString(token) {
		return true
	}

	// Pattern: garbled text like "Tht'!" or "l f_D S S O N"
	garbledPattern := regexp.MustCompile(`[A-Za-z]'[!?.]|^[a-z]_[A-Z]`)
	if garbledPattern.MatchString(token) {
		return true
	}

	return false
}

// isBibleReference checks for Bible book abbreviations
func isBibleReference(token string) bool {
	refs := map[string]bool{
		"Heb.": true, "Jer.": true, "Tim.": true, "Jn.": true,
		"Mt.": true, "Lk.": true, "Acts": true, "Rom.": true,
		"Cor.": true, "Eph.": true, "Col.": true, "Pt.": true,
		"Rev.": true, "Ps.": true, "Deut.": true, "Eccl.": true,
		"Job": true, "KJV": true, "RSV": true, "NIV": true,
	}

	// Remove trailing punctuation for check
	clean := strings.TrimRight(token, ".,;:")
	return refs[clean] || refs[token]
}

// isRomanNumeral checks for Roman numerals
func isRomanNumeral(token string) bool {
	match, _ := regexp.MatchString(`^[IVXLCDM]+$`, strings.ToUpper(token))
	return match
}

// isNumeric checks if token is a number
func isNumeric(token string) bool {
	clean := strings.Trim(token, ".,;:-")
	if clean == "" {
		return false
	}
	for _, r := range clean {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isPunctuation checks if token is just punctuation
func isPunctuation(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isNewSection detects section headers
func isNewSection(line string) bool {
	patterns := []string{
		"^Introduction", "^Presentation", "^Conclusion", "^Theme:",
		"^Reason", "^Proof", "^Note:", "^Instruction",
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, line); matched {
			return true
		}
	}
	return false
}
