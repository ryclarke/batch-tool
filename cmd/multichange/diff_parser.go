package multichange

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DiffChunk represents a chunk of changes in a diff
type DiffChunk struct {
	OldStart int      // Starting line number in old file
	OldCount int      // Number of lines in old file
	NewStart int      // Starting line number in new file
	NewCount int      // Number of lines in new file
	Lines    []string // The actual diff lines (with +/- prefixes)
}

// DiffHunk represents the actual changes within a chunk
type DiffHunk struct {
	ContextBefore []string // Lines of context before the change
	OldLines      []string // Lines to be removed
	NewLines      []string // Lines to be added
	ContextAfter  []string // Lines of context after the change
}

// parseDiffBetweenFiles generates and parses a unified diff between two files
func parseDiffBetweenFiles(oldFile, newFile string) ([]DiffChunk, error) {
	// Generate unified diff using git diff or regular diff
	cmd := exec.Command("diff", "-u", oldFile, newFile)
	output, err := cmd.Output()

	// diff returns exit code 1 when files differ, which is expected
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			// This is expected when files differ
		} else {
			return nil, fmt.Errorf("diff command failed: %w", err)
		}
	}

	return parseDiffOutput(string(output))
}

// parseDiffOutput parses unified diff output into structured chunks
func parseDiffOutput(diffOutput string) ([]DiffChunk, error) {
	var chunks []DiffChunk
	lines := strings.Split(diffOutput, "\n")

	// Regex to match chunk headers like @@ -1,4 +1,4 @@
	chunkHeaderRe := regexp.MustCompile(`^@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@`)

	var currentChunk *DiffChunk

	for _, line := range lines {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			// Skip file headers
			continue
		}

		if matches := chunkHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous chunk if exists
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
			}

			// Parse chunk header
			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}

			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentChunk = &DiffChunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Lines:    []string{},
			}
		} else if currentChunk != nil {
			// Add line to current chunk
			currentChunk.Lines = append(currentChunk.Lines, line)
		}
	}

	// Don't forget the last chunk
	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}

	return chunks, nil
}

// applyDiffToFile applies a diff chunk to a target file by finding matching context
func applyDiffToFile(targetFile string, chunk DiffChunk) (bool, error) {
	// Read target file
	content, err := os.ReadFile(targetFile)
	if err != nil {
		return false, fmt.Errorf("failed to read target file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// For chunks with complex interleaved changes, apply them directly
	newLines, applied := applyChunkDirectly(lines, chunk)
	if !applied {
		return false, nil
	}

	// Write back to file (if not dry run)
	if !dryRun {
		newContent := strings.Join(newLines, "\n")
		if err := os.WriteFile(targetFile, []byte(newContent), 0644); err != nil {
			return false, fmt.Errorf("failed to write updated file: %w", err)
		}
	}

	return true, nil
}

// applyChunkDirectly applies a diff chunk by matching the pattern and applying changes
func applyChunkDirectly(targetLines []string, chunk DiffChunk) ([]string, bool) {
	// Extract just the context and old lines to find the matching location
	var searchPattern []string
	var replacements []lineReplacement

	i := 0
	for _, line := range chunk.Lines {
		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case ' ': // Context line
			searchPattern = append(searchPattern, line[1:])
		case '-': // Line to be removed
			searchPattern = append(searchPattern, line[1:])
			replacements = append(replacements, lineReplacement{
				index:     len(searchPattern) - 1,
				isRemoval: true,
			})
		case '+': // Line to be added - this will replace the previous - line
			if len(replacements) > 0 && replacements[len(replacements)-1].isRemoval && replacements[len(replacements)-1].newLine == "" {
				// This + line corresponds to the previous - line
				replacements[len(replacements)-1].newLine = line[1:]
			} else {
				// This is an insertion at the current position in the search pattern
				replacements = append(replacements, lineReplacement{
					index:     len(searchPattern), // Insert at current position, not after
					isRemoval: false,
					newLine:   line[1:],
				})
			}
		}
		i++
	}

	// Find the location where this pattern matches
	location := findPatternMatch(targetLines, searchPattern)
	if location == -1 {
		return targetLines, false
	}

	// Apply the replacements
	return applyReplacements(targetLines, location, searchPattern, replacements), true
}

type lineReplacement struct {
	index     int    // Index within the search pattern
	isRemoval bool   // True if this is a removal, false if insertion
	newLine   string // The new line content (empty for pure removals)
}

// findPatternMatch finds the best matching location for a pattern in target lines
func findPatternMatch(targetLines []string, pattern []string) int {
	if len(pattern) == 0 {
		return -1
	}

	bestMatch := -1
	bestScore := 0.0

	// Try each possible starting position
	for i := 0; i <= len(targetLines)-len(pattern); i++ {
		score := calculateLineMatchScore(targetLines[i:i+len(pattern)], pattern)
		if score > bestScore && score > 0.25 { // Lowered threshold for better matching with diverse codebases
			bestScore = score
			bestMatch = i
		}
	}

	return bestMatch
}

// applyReplacements applies the line replacements at the specified location
func applyReplacements(targetLines []string, location int, pattern []string, replacements []lineReplacement) []string {
	var result []string

	// Add everything before the match
	result = append(result, targetLines[:location]...)

	// Apply replacements within the matched section
	for i := range pattern {
		// First, check for insertions that should happen BEFORE this line
		for _, repl := range replacements {
			if repl.index == i && !repl.isRemoval {
				// This is an insertion that should happen before line i
				result = append(result, repl.newLine)
			}
		}

		// Then handle the current line
		hasReplacement := false
		for _, repl := range replacements {
			if repl.index == i && repl.isRemoval {
				if repl.newLine != "" {
					// This is a substitution
					result = append(result, repl.newLine)
				}
				// If it's a pure removal (isRemoval=true, newLine=""), we skip adding the line
				hasReplacement = true
				break
			}
		}

		if !hasReplacement {
			// No replacement for this line, keep the original from target
			result = append(result, targetLines[location+i])
		}
	}

	// Check for insertions that should happen after the last line
	for _, repl := range replacements {
		if repl.index == len(pattern) && !repl.isRemoval {
			result = append(result, repl.newLine)
		}
	}

	// Add everything after the match
	afterIndex := location + len(pattern)
	if afterIndex < len(targetLines) {
		result = append(result, targetLines[afterIndex:]...)
	}

	return result
}

// calculateLineMatchScore calculates similarity between two sets of lines
func calculateLineMatchScore(target []string, pattern []string) float64 {
	if len(target) != len(pattern) {
		return 0.0
	}

	matches := 0
	for i, patternLine := range pattern {
		targetLine := target[i]

		// Normalize whitespace for comparison
		patternNorm := strings.TrimSpace(patternLine)
		targetNorm := strings.TrimSpace(targetLine)

		if patternNorm == targetNorm {
			matches++
		} else if similarity := calculateLineSimilarity(patternNorm, targetNorm); similarity > 0.8 {
			matches++
		}
	}

	return float64(matches) / float64(len(pattern))
}

// calculateLineSimilarity calculates similarity between two lines
func calculateLineSimilarity(line1, line2 string) float64 {
	if line1 == line2 {
		return 1.0
	}

	// Simple similarity based on common words
	words1 := strings.Fields(line1)
	words2 := strings.Fields(line2)

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	commonWords := 0
	totalWords := len(words1)

	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 {
				commonWords++
				break
			}
		}
	}

	return float64(commonWords) / float64(totalWords)
}

// tryDiffBasedMatch attempts to apply changes using diff-based matching
func tryDiffBasedMatch(targetFile string, change Change, repoName string) (bool, error) {
	// First, try regular diff-based matching
	chunks, err := parseDiffBetweenFiles(change.OldFile, change.NewFile)
	if err != nil {
		return false, fmt.Errorf("failed to generate diff: %w", err)
	}

	if len(chunks) == 0 {
		return false, nil // No changes found
	}

	// Create backup before applying any changes (only if not dry run)
	backupCreated := false
	if !dryRun {
		if err := createBackup(targetFile, change.RelativePath, repoName); err != nil {
			// Log warning but continue - backup failure shouldn't stop the apply
			fmt.Printf("    ⚠️  Warning: Failed to create backup for %s: %v\n", targetFile, err)
		} else {
			backupCreated = true
		}
	}

	// Apply each chunk to the target file
	appliedChanges := 0
	for _, chunk := range chunks {
		if applied, err := applyDiffToFile(targetFile, chunk); err != nil {
			return false, fmt.Errorf("failed to apply diff chunk: %w", err)
		} else if applied {
			appliedChanges++
		}
	}

	// If regular diff-based matching succeeded, we're done
	if appliedChanges > 0 {
		return true, nil
	}

	// If regular diff-based matching failed and this is a configuration file,
	// try configuration-aware matching as a fallback
	if isConfigurationFile(targetFile) {
		// Only create backup if we haven't already created one
		if !dryRun && !backupCreated {
			if err := createBackup(targetFile, change.RelativePath, repoName); err != nil {
				fmt.Printf("    ⚠️  Warning: Failed to create backup for %s: %v\n", targetFile, err)
			}
		}
		return tryConfigurationAwareMatch(targetFile, change)
	}

	return false, nil
}

// isConfigurationFile determines if a file should be treated as a configuration file
func isConfigurationFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filepath.Base(filename))

	// Don't treat source code files as configuration files, even if they contain "config" in the name
	sourceCodeExtensions := []string{".go", ".java", ".py", ".js", ".ts", ".cpp", ".c", ".h"}
	for _, sourceExt := range sourceCodeExtensions {
		if ext == sourceExt {
			return false
		}
	}

	// Check file extensions for actual configuration files
	configExtensions := []string{".yaml", ".yml", ".json", ".toml", ".ini", ".conf", ".config", ".tmpl", ".template"}
	for _, configExt := range configExtensions {
		if ext == configExt {
			return true
		}
	}

	// Check filename patterns for configuration files (excluding source code)
	configPatterns := []string{"configmap", "values", "settings", "env"}
	for _, pattern := range configPatterns {
		if strings.Contains(base, pattern) && !strings.Contains(base, ".go") {
			return true
		}
	}

	return false
}

// tryConfigurationAwareMatch applies changes with configuration-aware logic
func tryConfigurationAwareMatch(targetFile string, change Change) (bool, error) {
	// Read all file contents
	oldContent, err := os.ReadFile(change.OldFile)
	if err != nil {
		return false, fmt.Errorf("failed to read old file: %w", err)
	}

	newContent, err := os.ReadFile(change.NewFile)
	if err != nil {
		return false, fmt.Errorf("failed to read new file: %w", err)
	}

	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		return false, fmt.Errorf("failed to read target file: %w", err)
	}

	// Parse the changes between old and new
	configChanges := analyzeConfigurationChanges(string(oldContent), string(newContent))
	if len(configChanges) == 0 {
		return false, nil
	}

	// Apply configuration changes to target
	updatedContent, applied := applyConfigurationChanges(string(targetContent), configChanges)
	if !applied {
		return false, nil
	}

	// Write the updated content (if not dry run)
	if !dryRun {
		if err := os.WriteFile(targetFile, []byte(updatedContent), 0644); err != nil {
			return false, fmt.Errorf("failed to write updated file: %w", err)
		}
	}

	return true, nil
}

// ConfigChange represents a structural change in a configuration file
type ConfigChange struct {
	Type        string   // "add_to_list", "modify_value", "add_section"
	Section     string   // The section path (e.g., "localfeatures.example-service.mcmp.io")
	AddedItems  []string // Items to add to a list
	OriginalKey string   // Original key name (for service-specific transformations)
	TargetKey   string   // Transformed key name for the target service
}

// analyzeConfigurationChanges identifies structural changes between old and new config
func analyzeConfigurationChanges(oldContent, newContent string) []ConfigChange {
	var changes []ConfigChange

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Look for list additions in YAML-style configuration
	changes = append(changes, findListAdditions(oldLines, newLines)...)

	// Look for new sections or modified values
	changes = append(changes, findSectionChanges(oldLines, newLines)...)

	return changes
}

// findListAdditions identifies when items are added to lists
func findListAdditions(oldLines, newLines []string) []ConfigChange {
	var changes []ConfigChange

	// Build a map of line content for the old file
	oldLineSet := make(map[string]bool)
	for _, line := range oldLines {
		oldLineSet[strings.TrimSpace(line)] = true
	}

	// Find lines that exist in new but not in old
	var addedLines []string
	var currentSection string

	for _, line := range newLines {
		trimmed := strings.TrimSpace(line)

		// Track the current section we're in, focusing on feature lists
		if strings.Contains(trimmed, "localfeatures:") {
			currentSection = "localfeatures"
		} else if currentSection == "localfeatures" && strings.Contains(trimmed, ".mcmp.io:") {
			// Found a service-specific feature section
			serviceName := strings.Split(trimmed, ".mcmp.io:")[0]
			serviceName = strings.TrimSpace(serviceName)
			currentSection = "localfeatures." + serviceName + ".mcmp.io"
		}

		// Check if this is a list item that was added
		if isYAMLListItem(trimmed) && !oldLineSet[trimmed] && currentSection != "" && strings.Contains(currentSection, "localfeatures") {
			addedLines = append(addedLines, trimmed)
		}
	}

	// Create change if we found added items
	if len(addedLines) > 0 && currentSection != "" {
		changes = append(changes, ConfigChange{
			Type:        "add_to_list",
			Section:     currentSection,
			AddedItems:  addedLines,
			OriginalKey: currentSection,
			TargetKey:   currentSection,
		})
	}

	return changes
}

// findSectionChanges identifies structural changes in sections
func findSectionChanges(oldLines, newLines []string) []ConfigChange {
	// This could be expanded to handle section additions, key changes, etc.
	return []ConfigChange{}
}

// isYAMLSection checks if a line defines a YAML section
func isYAMLSection(line string) bool {
	return strings.Contains(line, ":") && !strings.HasPrefix(strings.TrimSpace(line), "-")
}

// isYAMLListItem checks if a line is a YAML list item
func isYAMLListItem(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "- ")
}

// getIndentLevel returns the indentation level of a line
func getIndentLevel(line string) int {
	count := 0
	for _, char := range line {
		if char == ' ' {
			count++
		} else if char == '\t' {
			count += 4 // Treat tab as 4 spaces
		} else {
			break
		}
	}
	return count
}

// applyConfigurationChanges applies configuration changes to target content
func applyConfigurationChanges(targetContent string, changes []ConfigChange) (string, bool) {
	lines := strings.Split(targetContent, "\n")
	applied := false

	for _, change := range changes {
		if change.Type == "add_to_list" {
			if newLines, wasApplied := addItemsToList(lines, change); wasApplied {
				lines = newLines
				applied = true
			}
		}
	}

	return strings.Join(lines, "\n"), applied
}

// addItemsToList adds items to a YAML list in the target content
func addItemsToList(lines []string, change ConfigChange) ([]string, bool) {
	// For localfeatures, we need special handling to find the right service section
	if strings.Contains(change.Section, "localfeatures") {
		return addItemsToLocalFeaturesList(lines, change)
	}

	// Find the section in the target file
	sectionIndex := findYAMLSection(lines, change.Section)
	if sectionIndex == -1 {
		return lines, false
	}

	// Find the end of the list in this section
	listEndIndex := findListEnd(lines, sectionIndex)
	if listEndIndex == -1 {
		return lines, false
	}

	// Get the indentation level of existing list items
	indent := getListItemIndent(lines, sectionIndex, listEndIndex)

	// Prepare new items with proper indentation
	var newItems []string
	for _, item := range change.AddedItems {
		// Remove the "- " prefix and add proper indentation
		itemContent := strings.TrimPrefix(strings.TrimSpace(item), "- ")
		newLine := strings.Repeat(" ", indent) + "- " + itemContent
		newItems = append(newItems, newLine)
	}

	// Insert new items at the end of the list
	result := make([]string, 0, len(lines)+len(newItems))
	result = append(result, lines[:listEndIndex+1]...)
	result = append(result, newItems...)
	result = append(result, lines[listEndIndex+1:]...)

	return result, true
}

// addItemsToLocalFeaturesList handles adding items to localfeatures lists with service name awareness
func addItemsToLocalFeaturesList(lines []string, change ConfigChange) ([]string, bool) {
	// Find any .mcmp.io section in the target file (could be different service)
	targetSectionIndex := -1
	for i, line := range lines {
		if strings.Contains(strings.TrimSpace(line), ".mcmp.io:") {
			targetSectionIndex = i
			break
		}
	}

	if targetSectionIndex == -1 {
		return lines, false
	}

	// Find the end of the list in this section
	listEndIndex := findListEnd(lines, targetSectionIndex)
	if listEndIndex == -1 {
		return lines, false
	}

	// Get the indentation level of existing list items
	indent := getListItemIndent(lines, targetSectionIndex, listEndIndex)

	// Prepare new items with proper indentation
	var newItems []string
	for _, item := range change.AddedItems {
		// Remove the "- " prefix and add proper indentation
		itemContent := strings.TrimPrefix(strings.TrimSpace(item), "- ")
		newLine := strings.Repeat(" ", indent) + "- " + itemContent
		newItems = append(newItems, newLine)
	}

	// Insert new items at the end of the list
	result := make([]string, 0, len(lines)+len(newItems))
	result = append(result, lines[:listEndIndex+1]...)
	result = append(result, newItems...)
	result = append(result, lines[listEndIndex+1:]...)

	return result, true
}

// findYAMLSection finds the line index of a YAML section, with special handling for localfeatures
func findYAMLSection(lines []string, sectionPath string) int {
	// Special handling for localfeatures sections
	if strings.Contains(sectionPath, "localfeatures") {
		return findLocalFeaturesSection(lines, sectionPath)
	}

	pathParts := strings.Split(sectionPath, ".")

	currentLevel := 0
	foundParts := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if isYAMLSection(trimmed) {
			key := strings.Split(trimmed, ":")[0]
			indent := getIndentLevel(line)

			// Check if this matches the current part we're looking for
			if foundParts < len(pathParts) && key == pathParts[foundParts] {
				if foundParts == 0 || indent > currentLevel {
					foundParts++
					currentLevel = indent

					// If we found all parts, return this index
					if foundParts == len(pathParts) {
						return i
					}
				}
			} else if indent <= currentLevel && foundParts > 0 {
				// We've moved to a different section at the same or higher level
				// Reset our search
				foundParts = 0
				currentLevel = 0

				// Check if this line starts a new matching path
				if key == pathParts[0] {
					foundParts = 1
					currentLevel = indent
				}
			}
		}
	}

	return -1
}

// findLocalFeaturesSection finds localfeatures sections in YAML config files
func findLocalFeaturesSection(lines []string, sectionPath string) int {
	// Look for localfeatures: first
	localFeaturesIndex := -1
	for i, line := range lines {
		if strings.Contains(strings.TrimSpace(line), "localfeatures:") {
			localFeaturesIndex = i
			break
		}
	}

	if localFeaturesIndex == -1 {
		return -1
	}

	// If we're just looking for "localfeatures", return that index
	if sectionPath == "localfeatures" {
		return localFeaturesIndex
	}

	// Otherwise, look for the service-specific section under localfeatures
	// Pattern: "localfeatures.{service}.mcmp.io"
	if strings.Contains(sectionPath, ".mcmp.io") {
		serviceName := strings.Split(sectionPath, ".mcmp.io")[0]
		serviceName = strings.Replace(serviceName, "localfeatures.", "", 1)

		// Look for the service section after localfeatures
		for i := localFeaturesIndex + 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if strings.Contains(line, serviceName+".mcmp.io:") {
				return i
			}
		}
	}

	return -1
}

// findListEnd finds the last line of a YAML list starting from a section
func findListEnd(lines []string, sectionStart int) int {
	listIndent := -1
	lastListItem := -1

	for i := sectionStart + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := getIndentLevel(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		if isYAMLListItem(trimmed) {
			if listIndent == -1 {
				listIndent = indent
			}
			if indent == listIndent {
				lastListItem = i
			}
		} else if isYAMLSection(trimmed) && indent <= listIndent {
			// We've hit a new section, end of list
			break
		}
	}

	return lastListItem
}

// getListItemIndent gets the indentation level for list items in a section
func getListItemIndent(lines []string, sectionStart, listEnd int) int {
	for i := sectionStart + 1; i <= listEnd; i++ {
		if i < len(lines) && isYAMLListItem(strings.TrimSpace(lines[i])) {
			return getIndentLevel(lines[i])
		}
	}
	return 8 // Default indentation
}
