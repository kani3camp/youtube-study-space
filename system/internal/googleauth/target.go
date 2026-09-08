package googleauth

import "fmt"

const (
	DevelopmentProjectID = "test-youtube-study-space"
	ProductionProjectID  = "youtube-study-space"
)

func ProjectIDForEnvironment(environment string) (string, error) {
	switch environment {
	case "development":
		return DevelopmentProjectID, nil
	case "production":
		return ProductionProjectID, nil
	default:
		return "", fmt.Errorf("environment must be development or production: %q", environment)
	}
}
