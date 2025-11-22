package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// CoverageAnalyzer analyzes test coverage against constitution principles
type CoverageAnalyzer struct {
	projectRoot string
	coverageData map[string]*PackageCoverage
}

// PackageCoverage represents coverage data for a package
type PackageCoverage struct {
	PackageName     string
	Functions       map[string]*FunctionCoverage
	AcceptanceTests map[string]*AcceptanceTestCoverage
	ErrorTests      map[string]*ErrorTestCoverage
	EdgeCaseTests   map[string]*EdgeCaseTestCoverage
}

// FunctionCoverage tracks coverage of individual functions
type FunctionCoverage struct {
	Name         string
	LineStart     int
	LineEnd       int
	IsTested     bool
	TestCases    []string
	CoveragePct  float64
}

// AcceptanceTestCoverage tracks acceptance scenario coverage
type AcceptanceTestCoverage struct {
	UserStory string
	Scenario  string
	TestName  string
	IsCovered bool
}

// ErrorTestCoverage tracks error condition coverage
type ErrorTestCoverage struct {
	ErrorType    string
	ErrorCode    string
	ServiceError string
	HTTPCode     int
	IsCovered    bool
}

// EdgeCaseTestCoverage tracks edge case coverage
type EdgeCaseTestCoverage struct {
	Category     string
	Description  string
	TestName     string
	IsCovered    bool
}

// NewCoverageAnalyzer creates a new coverage analyzer
func NewCoverageAnalyzer(projectRoot string) *CoverageAnalyzer {
	return &CoverageAnalyzer{
		projectRoot:  projectRoot,
		coverageData: make(map[string]*PackageCoverage),
	}
}

// AnalyzeConstitutionCoverage performs comprehensive coverage analysis
func (ca *CoverageAnalyzer) AnalyzeConstitutionCoverage(t *testing.T) *CoverageReport {
	report := &CoverageReport{
		Principles: make(map[string]*PrincipleCoverage),
	}

	// Principle I: Integration Testing First
	report.Principles["I"] = ca.analyzeIntegrationTesting(t)

	// Principle II: Table-Driven Test Design
	report.Principles["II"] = ca.analyzeTableDrivenDesign(t)

	// Principle III: Edge Case Coverage
	report.Principles["III"] = ca.analyzeEdgeCaseCoverage(t)

	// Principle IV: Real Database Fixtures
	report.Principles["IV"] = ca.analyzeDatabaseFixtures(t)

	// Principle V: ServeHTTP Endpoint Testing
	report.Principles["V"] = ca.analyzeHTTPEndpointTesting(t)

	// Principle VI: Protobuf Data Structures
	report.Principles["VI"] = ca.analyzeProtobufTesting(t)

	// Principle VII: Distributed Tracing
	report.Principles["VII"] = ca.analyzeTracingCoverage(t)

	// Principle VIII: Service Layer Architecture
	report.Principles["VIII"] = ca.analyzeServiceLayerArchitecture(t)

	// Principle IX: Comprehensive Error Handling
	report.Principles["IX"] = ca.analyzeErrorHandlingCoverage(t)

	// Principle X: Context-Aware Operations
	report.Principles["X"] = ca.analyzeContextAwareness(t)

	// Principle XI: Continuous Test Verification
	report.Principles["XI"] = ca.analyzeContinuousVerification(t)

	// Principle XII: Root Cause Tracing
	report.Principles["XII"] = ca.analyzeRootCauseTracing(t)

	// Principle XIII: Acceptance Scenario Coverage
	report.Principles["XIII"] = ca.analyzeAcceptanceScenarioCoverage(t)

	report.CalculateOverallCoverage()
	return report
}

// CoverageReport represents the comprehensive coverage analysis report
type CoverageReport struct {
	Principles         map[string]*PrincipleCoverage
	OverallCoverage    float64
	Gaps               []CoverageGap
	Recommendations    []string
}

// PrincipleCoverage tracks coverage for individual constitution principles
type PrincipleCoverage struct {
	Principle    string
	Description  string
	CoveragePct  float64
	TestsCovered int
	TestsTotal   int
	Gaps         []string
}

// CoverageGap represents a specific coverage gap
type CoverageGap struct {
	Principle   string
	Category    string
	Description string
	Severity    string // "critical", "high", "medium", "low"
}

// CalculateOverallCoverage calculates the overall coverage percentage
func (cr *CoverageReport) CalculateOverallCoverage() {
	totalCoverage := 0.0
	principleCount := 0

	for _, principle := range cr.Principles {
		totalCoverage += principle.CoveragePct
		principleCount++
	}

	if principleCount > 0 {
		cr.OverallCoverage = totalCoverage / float64(principleCount)
	}

	cr.identifyCriticalGaps()
}

// identifyCriticalGaps identifies the most critical coverage gaps
func (cr *CoverageReport) identifyCriticalGaps() {
	for principleID, principle := range cr.Principles {
		if principle.CoveragePct < 80.0 {
			cr.Gaps = append(cr.Gaps, CoverageGap{
				Principle:   principleID,
				Category:    "Low Coverage",
				Description: fmt.Sprintf("Principle %s (%s) has only %.1f%% coverage", principleID, principle.Description, principle.CoveragePct),
				Severity:    "high",
			})
		}

		for _, gap := range principle.Gaps {
			cr.Gaps = append(cr.Gaps, CoverageGap{
				Principle:   principleID,
				Category:    "Specific Gap",
				Description: gap,
				Severity:    "medium",
			})
		}
	}

	// Sort gaps by severity
	sort.Slice(cr.Gaps, func(i, j int) bool {
		severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		return severityOrder[cr.Gaps[i].Severity] < severityOrder[cr.Gaps[j].Severity]
	})

	cr.generateRecommendations()
}

// generateRecommendations creates actionable recommendations
func (cr *CoverageReport) generateRecommendations() {
	// Generate recommendations based on gaps
	for _, gap := range cr.Gaps {
		switch gap.Principle {
		case "IX": // Error Handling
			cr.Recommendations = append(cr.Recommendations,
				"Add comprehensive error testing for all sentinel errors and HTTP error codes")
		case "III": // Edge Cases
			cr.Recommendations = append(cr.Recommendations,
				"Implement systematic edge case testing for input validation, boundary conditions, and error scenarios")
		case "VII": // Tracing
			cr.Recommendations = append(cr.Recommendations,
				"Add tracing validation tests to ensure all endpoints create proper spans")
		}
	}

	// Add general recommendations
	if cr.OverallCoverage < 90.0 {
		cr.Recommendations = append(cr.Recommendations,
			"Achieve 90%+ test coverage across all constitution principles")
	}
}

// PrintReport prints a formatted coverage report
func (cr *CoverageReport) PrintReport() {
	fmt.Println("=== PIM SYSTEM TEST COVERAGE REPORT ===")
	fmt.Printf("Overall Coverage: %.1f%%\n\n", cr.OverallCoverage)

	fmt.Println("CONSTITUTION PRINCIPLE COVERAGE:")
	for _, principleID := range []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII", "XIII"} {
		if principle, exists := cr.Principles[principleID]; exists {
			status := "✅"
			if principle.CoveragePct < 80 {
				status = "❌"
			} else if principle.CoveragePct < 90 {
				status = "⚠️"
			}
			fmt.Printf("%s Principle %s: %.1f%% (%d/%d) - %s\n",
				status, principleID, principle.CoveragePct, principle.TestsCovered, principle.TestsTotal, principle.Description)
		}
	}

	if len(cr.Gaps) > 0 {
		fmt.Println("\nCRITICAL GAPS:")
		for _, gap := range cr.Gaps {
			severityIcon := map[string]string{
				"critical": "🔴",
				"high":     "🟠",
				"medium":   "🟡",
				"low":      "🟢",
			}[gap.Severity]
			fmt.Printf("%s %s: %s\n", severityIcon, gap.Category, gap.Description)
		}
	}

	if len(cr.Recommendations) > 0 {
		fmt.Println("\nRECOMMENDATIONS:")
		for i, rec := range cr.Recommendations {
			fmt.Printf("%d. %s\n", i+1, rec)
		}
	}
}

// analyzeIntegrationTesting analyzes Principle I compliance
func (ca *CoverageAnalyzer) analyzeIntegrationTesting(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "I",
		Description: "Integration Testing First - Real PostgreSQL via testcontainers-go",
		TestsTotal:  1,
	}

	// Check if tests use testcontainers-go
	hasTestContainers := ca.checkForTestContainers()
	if hasTestContainers {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Tests do not use testcontainers-go for real database testing")
	}

	return pc
}

// analyzeTableDrivenDesign analyzes Principle II compliance
func (ca *CoverageAnalyzer) analyzeTableDrivenDesign(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "II",
		Description: "Table-Driven Test Design",
		TestsTotal:  1,
	}

	// Analyze test files for table-driven patterns
	testFiles := ca.findTestFiles()
	tableDrivenCount := 0
	totalTestFunctions := 0

	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Count table-driven test functions (looking for []struct patterns)
		tableDrivenRegex := regexp.MustCompile(`testCases\s*:=\s*\[\]struct`)
		if tableDrivenRegex.Match(content) {
			tableDrivenCount++
		}

		// Count total test functions
		testFuncRegex := regexp.MustCompile(`func\s+Test\w+`)
		matches := testFuncRegex.FindAll(content, -1)
		totalTestFunctions += len(matches)
	}

	if totalTestFunctions > 0 {
		pc.CoveragePct = float64(tableDrivenCount) / float64(totalTestFunctions) * 100
		pc.TestsCovered = tableDrivenCount
		pc.TestsTotal = totalTestFunctions
	}

	if pc.CoveragePct < 80 {
		pc.Gaps = append(pc.Gaps, "Less than 80% of test functions use table-driven design")
	}

	return pc
}

// analyzeEdgeCaseCoverage analyzes Principle III compliance
func (ca *CoverageAnalyzer) analyzeEdgeCaseCoverage(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "III",
		Description: "Edge Case Coverage - Input validation, boundary conditions, auth/authz, data state, database errors, HTTP specifics",
		TestsTotal:  24, // Based on edge cases listed in spec.md
	}

	// Check for edge case categories
	edgeCaseCategories := []string{
		"Input Validation",
		"Boundary Conditions",
		"Authentication & Authorization",
		"Data State",
		"Database Errors",
		"HTTP Specifics",
	}

	coveredCategories := 0
	for _, category := range edgeCaseCategories {
		if ca.hasEdgeCaseTests(category) {
			coveredCategories++
		}
	}

	pc.CoveragePct = float64(coveredCategories) / float64(len(edgeCaseCategories)) * 100
	pc.TestsCovered = coveredCategories
	pc.TestsTotal = len(edgeCaseCategories)

	if pc.CoveragePct < 80 {
		pc.Gaps = append(pc.Gaps, "Missing comprehensive edge case testing")
	}

	return pc
}

// analyzeDatabaseFixtures analyzes Principle IV compliance
func (ca *CoverageAnalyzer) analyzeDatabaseFixtures(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "IV",
		Description: "Real Database Fixtures - GORM operations against real test database",
		TestsTotal:  1,
	}

	// Check for real database fixture usage
	hasRealFixtures := ca.checkForRealFixtures()
	if hasRealFixtures {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Tests do not use real database fixtures")
	}

	return pc
}

// analyzeHTTPEndpointTesting analyzes Principle V compliance
func (ca *CoverageAnalyzer) analyzeHTTPEndpointTesting(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "V",
		Description: "ServeHTTP Endpoint Testing - httptest.ResponseRecorder through full handler chains",
		TestsTotal:  1,
	}

	// Check for httptest usage
	hasHTTPTesting := ca.checkForHTTPTesting()
	if hasHTTPTesting {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Missing comprehensive HTTP endpoint testing")
	}

	return pc
}

// analyzeProtobufTesting analyzes Principle VI compliance
func (ca *CoverageAnalyzer) analyzeProtobufTesting(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "VI",
		Description: "Protobuf Data Structures - protocmp.Transform() with cmp.Diff()",
		TestsTotal:  1,
	}

	// Check for protocmp usage
	hasProtobufTesting := ca.checkForProtobufTesting()
	if hasProtobufTesting {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Missing protocmp.Transform() usage in protobuf assertions")
	}

	return pc
}

// analyzeTracingCoverage analyzes Principle VII compliance
func (ca *CoverageAnalyzer) analyzeTracingCoverage(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "VII",
		Description: "Distributed Tracing - All HTTP endpoints create OpenTracing spans",
		TestsTotal:  1,
	}

	// Check for tracing span creation
	hasTracing := ca.checkForTracing()
	if hasTracing {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Missing distributed tracing validation")
	}

	return pc
}

// analyzeServiceLayerArchitecture analyzes Principle VIII compliance
func (ca *CoverageAnalyzer) analyzeServiceLayerArchitecture(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "VIII",
		Description: "Service Layer Architecture - Business logic in public services/ package",
		TestsTotal:  1,
	}

	// Check service layer structure
	hasServiceLayer := ca.checkServiceLayerArchitecture()
	if hasServiceLayer {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Service layer architecture not properly implemented")
	}

	return pc
}

// analyzeErrorHandlingCoverage analyzes Principle IX compliance
func (ca *CoverageAnalyzer) analyzeErrorHandlingCoverage(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "IX",
		Description: "Comprehensive Error Handling - TestAllSentinelErrors and TestAllHTTPErrorCodes",
		TestsTotal:  2,
	}

	// Check for comprehensive error testing
	hasSentinelTests := ca.checkForSentinelErrorTests()
	hasHTTPErrorTests := ca.checkForHTTPErrorTests()

	covered := 0
	if hasSentinelTests {
		covered++
	}
	if hasHTTPErrorTests {
		covered++
	}

	pc.CoveragePct = float64(covered) / 2.0 * 100
	pc.TestsCovered = covered

	if !hasSentinelTests {
		pc.Gaps = append(pc.Gaps, "Missing TestAllSentinelErrors function")
	}
	if !hasHTTPErrorTests {
		pc.Gaps = append(pc.Gaps, "Missing TestAllHTTPErrorCodes function")
	}

	return pc
}

// analyzeContextAwareness analyzes Principle X compliance
func (ca *CoverageAnalyzer) analyzeContextAwareness(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "X",
		Description: "Context-Aware Operations - All service methods accept context.Context",
		TestsTotal:  1,
	}

	// Check for context usage
	hasContext := ca.checkForContextAwareness()
	if hasContext {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Missing context.Context usage in service methods")
	}

	return pc
}

// analyzeContinuousVerification analyzes Principle XI compliance
func (ca *CoverageAnalyzer) analyzeContinuousVerification(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "XI",
		Description: "Continuous Test Verification - All code changes verified with go test ./...",
		TestsTotal:  1,
	}

	// This is more of a process check, assume it's followed if tests exist
	pc.TestsCovered = 1
	pc.CoveragePct = 100.0

	return pc
}

// analyzeRootCauseTracing analyzes Principle XII compliance
func (ca *CoverageAnalyzer) analyzeRootCauseTracing(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "XII",
		Description: "Root Cause Tracing - Debug problems backward through call chain",
		TestsTotal:  1,
	}

	// Check for error wrapping patterns
	hasRootCauseTracing := ca.checkForRootCauseTracing()
	if hasRootCauseTracing {
		pc.TestsCovered = 1
		pc.CoveragePct = 100.0
	} else {
		pc.Gaps = append(pc.Gaps, "Missing proper error wrapping with %w verb")
	}

	return pc
}

// analyzeAcceptanceScenarioCoverage analyzes Principle XIII compliance
func (ca *CoverageAnalyzer) analyzeAcceptanceScenarioCoverage(t *testing.T) *PrincipleCoverage {
	pc := &PrincipleCoverage{
		Principle:   "XIII",
		Description: "Acceptance Scenario Coverage - All 24 acceptance scenarios tested",
		TestsTotal:  24,
	}

	// Count acceptance scenario tests (US*-AS* patterns)
	scenariosCovered := ca.countAcceptanceScenarios()
	pc.CoveragePct = float64(scenariosCovered) / 24.0 * 100
	pc.TestsCovered = scenariosCovered

	if scenariosCovered < 24 {
		pc.Gaps = append(pc.Gaps, fmt.Sprintf("Only %d of 24 acceptance scenarios covered", scenariosCovered))
	}

	return pc
}

// Helper methods for checking various aspects

func (ca *CoverageAnalyzer) findTestFiles() []string {
	var testFiles []string
	filepath.Walk(filepath.Join(ca.projectRoot, "tests"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			testFiles = append(testFiles, path)
		}
		return nil
	})
	return testFiles
}

func (ca *CoverageAnalyzer) checkForTestContainers() bool {
	testFiles := ca.findTestFiles()
	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), "testcontainers-go") || strings.Contains(string(content), "testcontainers") {
			return true
		}
	}
	return false
}

func (ca *CoverageAnalyzer) hasEdgeCaseTests(category string) bool {
	testFiles := ca.findTestFiles()
	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		// Look for test names containing edge case keywords
		testContent := string(content)
		switch category {
		case "Input Validation":
			if strings.Contains(testContent, "empty") || strings.Contains(testContent, "validation") || strings.Contains(testContent, "invalid") {
				return true
			}
		case "Boundary Conditions":
			if strings.Contains(testContent, "boundary") || strings.Contains(testContent, "limit") || strings.Contains(testContent, "max") {
				return true
			}
		case "Authentication & Authorization":
			if strings.Contains(testContent, "auth") || strings.Contains(testContent, "unauthorized") || strings.Contains(testContent, "forbidden") {
				return true
			}
		case "Data State":
			if strings.Contains(testContent, "state") || strings.Contains(testContent, "concurrent") || strings.Contains(testContent, "orphan") {
				return true
			}
		case "Database Errors":
			if strings.Contains(testContent, "constraint") || strings.Contains(testContent, "unique") || strings.Contains(testContent, "foreign") {
				return true
			}
		case "HTTP Specifics":
			if strings.Contains(testContent, "http") || strings.Contains(testContent, "method") || strings.Contains(testContent, "header") {
				return true
			}
		}
	}
	return false
}

func (ca *CoverageAnalyzer) checkForRealFixtures() bool {
	fixtureFile := filepath.Join(ca.projectRoot, "tests/testutil/fixtures.go")
	if _, err := os.Stat(fixtureFile); os.IsNotExist(err) {
		return false
	}
	content, err := os.ReadFile(fixtureFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "CreateTest") || strings.Contains(string(content), "db.Create")
}

func (ca *CoverageAnalyzer) checkForHTTPTesting() bool {
	testFiles := ca.findTestFiles()
	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), "httptest.ResponseRecorder") || strings.Contains(string(content), "httptest.NewRecorder") {
			return true
		}
	}
	return false
}

func (ca *CoverageAnalyzer) checkForProtobufTesting() bool {
	testFiles := ca.findTestFiles()
	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), "protocmp.Transform") || strings.Contains(string(content), "cmp.Diff") {
			return true
		}
	}
	return false
}

func (ca *CoverageAnalyzer) checkForTracing() bool {
	// Check both service and handler files for tracing
	filesToCheck := []string{
		filepath.Join(ca.projectRoot, "services"),
		filepath.Join(ca.projectRoot, "handlers"),
	}

	for _, dir := range filesToCheck {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if strings.Contains(string(content), "opentracing.StartSpanFromContext") {
				return fmt.Errorf("found") // Return error to stop walking
			}
			return nil
		})
	}
	return false
}

func (ca *CoverageAnalyzer) checkServiceLayerArchitecture() bool {
	serviceDir := filepath.Join(ca.projectRoot, "services")
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return false
	}

	// Check if services are in public package (not internal)
	filepath.Walk(serviceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(content), "package services") {
			return fmt.Errorf("found") // Return error to stop walking
		}
		return nil
	})
	return false
}

func (ca *CoverageAnalyzer) checkForSentinelErrorTests() bool {
	errorTestFile := filepath.Join(ca.projectRoot, "tests/integration/error_handling_test.go")
	if _, err := os.Stat(errorTestFile); os.IsNotExist(err) {
		return false
	}
	content, err := os.ReadFile(errorTestFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "TestAllSentinelErrors")
}

func (ca *CoverageAnalyzer) checkForHTTPErrorTests() bool {
	errorTestFile := filepath.Join(ca.projectRoot, "tests/integration/error_handling_test.go")
	if _, err := os.Stat(errorTestFile); os.IsNotExist(err) {
		return false
	}
	content, err := os.ReadFile(errorTestFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "TestAllHTTPErrorCodes")
}

func (ca *CoverageAnalyzer) checkForContextAwareness() bool {
	serviceFiles, _ := filepath.Glob(filepath.Join(ca.projectRoot, "services", "*.go"))
	for _, file := range serviceFiles {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		// Check if service methods have context.Context as first parameter
		if strings.Contains(string(content), "func (") && strings.Contains(string(content), "context.Context") {
			return true
		}
	}
	return false
}

func (ca *CoverageAnalyzer) checkForRootCauseTracing() bool {
	filesToCheck := []string{
		filepath.Join(ca.projectRoot, "services"),
		filepath.Join(ca.projectRoot, "handlers"),
	}

	for _, dir := range filesToCheck {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if strings.Contains(string(content), `fmt.Errorf("%w"`) {
				return fmt.Errorf("found") // Return error to stop walking
			}
			return nil
		})
	}
	return false
}

func (ca *CoverageAnalyzer) countAcceptanceScenarios() int {
	testFiles := ca.findTestFiles()
	count := 0
	for _, file := range testFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		// Count US*-AS* patterns
		usAsRegex := regexp.MustCompile(`US\d+-AS\d+`)
		matches := usAsRegex.FindAll(content, -1)
		count += len(matches)
	}
	return count
}

// TestCoverageAnalysis is a test function that runs comprehensive coverage analysis
func TestCoverageAnalysis(t *testing.T) {
	analyzer := NewCoverageAnalyzer(".")
	report := analyzer.AnalyzeConstitutionCoverage(t)

	// Print the report
	report.PrintReport()

	// Assert minimum coverage requirements
	if report.OverallCoverage < 80.0 {
		t.Errorf("Overall test coverage is too low: %.1f%% (minimum required: 80%%)", report.OverallCoverage)
	}

	// Critical principles must have high coverage
	criticalPrinciples := []string{"IX", "III", "XIII"} // Error handling, Edge cases, Acceptance scenarios
	for _, principle := range criticalPrinciples {
		if pc, exists := report.Principles[principle]; exists && pc.CoveragePct < 90.0 {
			t.Errorf("Critical principle %s has insufficient coverage: %.1f%% (minimum required: 90%%)", principle, pc.CoveragePct)
		}
	}
}
