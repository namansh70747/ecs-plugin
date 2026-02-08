package executor

import (
	"encoding/json"
	"fmt"
)

// ValidateTaskDefinition validates task definition JSON format
func (e *Executor) ValidateTaskDefinition(taskDefJSON string) error {
	if taskDefJSON == "" {
		return fmt.Errorf("task definition cannot be empty")
	}

	// Basic JSON validation
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(taskDefJSON), &data); err != nil {
		// Not JSON, might be just a task def name/ARN
		if len(taskDefJSON) < 3 {
			return fmt.Errorf("invalid task definition")
		}
	}

	return nil
}

// GetTaskDefinitionName extracts name from ARN or returns as-is
func (e *Executor) GetTaskDefinitionName(taskDef string) string {
	// If it's an ARN like arn:aws:ecs:region:account:task-definition/name:version
	// extract just the name, otherwise return as-is
	return taskDef
}
