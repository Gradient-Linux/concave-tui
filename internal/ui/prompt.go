package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Confirm prompts the user for a yes or no answer.
func Confirm(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	Info("Prompt", question+" [y/N]")

	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// Checklist prompts the user to select comma-separated item indexes.
func Checklist(items []string) []string {
	reader := bufio.NewReader(os.Stdin)
	Header("Select one or more items")
	for idx, item := range items {
		Info(strconv.Itoa(idx+1), item)
	}
	Info("Prompt", "Enter comma-separated numbers and press Enter")

	line, err := reader.ReadString('\n')
	if err != nil {
		return nil
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	seen := map[int]struct{}{}
	var selected []string
	for _, raw := range strings.Split(line, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n < 1 || n > len(items) {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		selected = append(selected, items[n-1])
	}

	if len(selected) == 0 {
		Fail("Prompt", fmt.Sprintf("no valid selection from %q", line))
	}

	return selected
}
