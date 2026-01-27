package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

func main() {
	inputPath := "/home/chaschel/Documents/ibm/ai/bchat/docs/templates/examples/inc/funda_beliefs.txt"
	qaOutputPath := "/home/chaschel/Documents/ibm/ai/bchat/docs/templates/examples/inc/QA.txt"

	// Read input file
	content, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")

	var qaPairs []string

	var currentQ, currentA string
	inAnswer := false

	for _, line := range lines {
		// Clean the line (remove corrupted tokens)
		cleanedLine := cleanLine(line)

		// Extract Q&A pairs
		trimmed := strings.TrimSpace(cleanedLine)

		if strings.HasPrefix(trimmed, "Q.") {
			// Save previous Q&A pair if exists
			if currentQ != "" && currentA != "" {
				qaPairs = append(qaPairs, fmt.Sprintf("%s\n%s\n", currentQ, currentA))
			}
			currentQ = trimmed
			currentA = ""
			inAnswer = false
		} else if strings.HasPrefix(trimmed, "A.") {
			currentA = trimmed
			inAnswer = true
		} else if inAnswer && trimmed != "" && !strings.HasPrefix(trimmed, "Q.") {
			// Continue capturing multi-line answer until next Q or blank
			if !isNewSection(trimmed) {
				currentA += " " + trimmed
			}
		}
	}

	// Don't forget last Q&A pair
	if currentQ != "" && currentA != "" {
		qaPairs = append(qaPairs, fmt.Sprintf("%s\n%s\n", currentQ, currentA))
	}

	// Write QA.txt
	qaFile, err := os.Create(qaOutputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating QA file: %v\n", err)
		os.Exit(1)
	}
	defer qaFile.Close()

	for _, qa := range qaPairs {
		qaFile.WriteString(qa + "\n")
	}

	fmt.Printf("Extracted %d Q&A pairs to %s\n", len(qaPairs), qaOutputPath)
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
