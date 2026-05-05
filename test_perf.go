package main

import (
	"fmt"
	"regexp"
	"time"
	"strings"
)

func testRegex(name, pattern string, size int) {
	testContent := strings.Repeat("a", size)

	regex, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Printf("%s: COMPILE ERROR: %v\n", name, err)
		return
	}

	start := time.Now()
	match := regex.FindStringIndex(testContent)
	elapsed := time.Since(start)

	fmt.Printf("%-20s: %10v (match: %v)\n", name, elapsed, match != nil)
}

func main() {
	fmt.Println("Testing 1MB of 'a' characters (worst case for .* patterns):")
	testRegex("SKIMMER-003", `cc_number.*mail\(|mail\(.*cc_number`, 1048576)
	testRegex("SKIMMER-005", `document\.forms.*querySelector|querySelector.*document\.forms`, 1048576)
	testRegex("SKIMMER-013", `checkout.*payment.*intercept|intercept.*payment.*checkout`, 1048576)
	testRegex("SKIMMER-015", `addEventListener.*keypress|addEventListener.*keydown|onkeypress`, 1048576)
	testRegex("OBFUSC-001", `[A-Za-z0-9+/=]{500,2000}`, 1048576)
	testRegex("OBFUSC-010", `\$\w+\s*\^\s*\$\w+.*eval|\beval\b.*\$\w+\s*\^\s*\$\w+`, 1048576)
	testRegex("MAGENTO-004", `fopen.*\.(jpg|png|gif).*(payment|cc)|(?:payment|cc).*fopen.*\.(jpg|png|gif)`, 1048576)
	testRegex("MAGENTO-008", `crontab|/etc/cron\.|schedule.*backdoor`, 1048576)
	testRegex("MAGENTO-010", `Mage::getConfig\(\).*decrypt|decrypt.*password`, 1048576)
	testRegex("MAGENTO-012", `oauth_token.*secret|admin.*token.*bearer`, 1048576)
}
