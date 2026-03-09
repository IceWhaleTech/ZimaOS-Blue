package workflow

// DefaultTemplates returns the built-in workflow templates.
func DefaultTemplates() []WorkflowTemplateResponse {
	return []WorkflowTemplateResponse{
		{
			ID:          "http-webhook",
			Name:        "HTTP Webhook Handler",
			Description: "Receive HTTP webhooks and process data",
			Category:    "integration",
			Tags:        []string{"webhook", "http", "api"},
			Workflow: Workflow{
				Name: "HTTP Webhook Handler",
				Nodes: []Node{
					{
						ID:   "trigger-1",
						Type: NodeTypeTrigger,
						Name: "Webhook Trigger",
						Config: map[string]interface{}{
							"trigger": map[string]interface{}{
								"type": "webhook",
								"webhook": map[string]interface{}{
									"path":   "/my-webhook",
									"method": "POST",
								},
							},
						},
					},
					{
						ID:   "action-1",
						Type: NodeTypeAction,
						Name: "Process Data",
						Config: map[string]interface{}{
							"type": "javascript",
							"javascript": map[string]interface{}{
								"code": "return { processed: true, data: input.body };",
							},
						},
					},
				},
				Connections: []Connection{{
					ID:         "conn-1",
					SourceNode: "trigger-1",
					TargetNode: "action-1",
				}},
			},
		},
		{
			ID:          "scheduled-task",
			Name:        "Scheduled Task",
			Description: "Run a task on a schedule",
			Category:    "automation",
			Tags:        []string{"schedule", "cron", "automation"},
			Workflow: Workflow{
				Name: "Scheduled Task",
				Nodes: []Node{
					{
						ID:   "trigger-1",
						Type: NodeTypeTrigger,
						Name: "Schedule Trigger",
						Config: map[string]interface{}{
							"trigger": map[string]interface{}{
								"type": "schedule",
								"schedule": map[string]interface{}{
									"cron": "0 0 * * * *",
								},
							},
						},
					},
					{
						ID:   "action-1",
						Type: NodeTypeAction,
						Name: "HTTP Request",
						Config: map[string]interface{}{
							"type": "http",
							"http": map[string]interface{}{
								"url":    "https://api.example.com/health",
								"method": "GET",
							},
						},
					},
				},
				Connections: []Connection{{
					ID:         "conn-1",
					SourceNode: "trigger-1",
					TargetNode: "action-1",
				}},
			},
		},
		{
			ID:          "ha-automation",
			Name:        "Home Assistant Automation",
			Description: "Automate Home Assistant based on state changes",
			Category:    "smart-home",
			Tags:        []string{"home-assistant", "smart-home", "automation"},
			Workflow: Workflow{
				Name: "Home Assistant Automation",
				Nodes: []Node{
					{
						ID:   "trigger-1",
						Type: NodeTypeTrigger,
						Name: "State Change Trigger",
						Config: map[string]interface{}{
							"trigger": map[string]interface{}{
								"type": "ha_state",
								"ha": map[string]interface{}{
									"entity_id": "binary_sensor.motion",
									"to_state":  "on",
								},
							},
						},
					},
					{
						ID:   "action-1",
						Type: NodeTypeAction,
						Name: "Turn On Light",
						Config: map[string]interface{}{
							"type": "ha_service",
							"ha": map[string]interface{}{
								"domain":    "light",
								"service":   "turn_on",
								"entity_id": "light.living_room",
							},
						},
					},
				},
				Connections: []Connection{{
					ID:         "conn-1",
					SourceNode: "trigger-1",
					TargetNode: "action-1",
				}},
			},
		},
	}
}
