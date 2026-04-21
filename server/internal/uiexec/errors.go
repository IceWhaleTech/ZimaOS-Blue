package uiexec

import "errors"

var (
	ErrPlannerUnavailable = errors.New("planner unavailable")
	ErrInvalidDSL         = errors.New("invalid action dsl")
)
