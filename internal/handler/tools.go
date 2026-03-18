// Package handler implements the MCP handlers for tools, resources, and prompts.
// This file contains the tools handler, which exposes the tenant's active skills
// as MCP Tools. Tenant identity is read from the request context per call.
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/tenant"
)

// BackendClientIface defines the backend operations used by the tools handler.
type BackendClientIface interface {
	ListActiveSkills(ctx context.Context) ([]backend.SkillDTO, error)
}

// SkillRuntimeClientIface defines the skill-runtime operations used by the tools handler.
type SkillRuntimeClientIface interface {
	InvokeSkill(ctx context.Context, req skillruntime.InvokeSkillRequest) (*skillruntime.SkillResult, error)
}

// ToolsHandlerImpl implements the ToolsHandler interface of the MCPServer.
// Converts AgentHub skills to the MCP Tool format and delegates executions to the skill-runtime.
type ToolsHandlerImpl struct {
	backendClient      BackendClientIface
	skillRuntimeClient SkillRuntimeClientIface
}

// NewToolsHandler creates a new MCP tools handler.
func NewToolsHandler(
	backendClient BackendClientIface,
	skillRuntimeClient SkillRuntimeClientIface,
) *ToolsHandlerImpl {
	return &ToolsHandlerImpl{
		backendClient:      backendClient,
		skillRuntimeClient: skillRuntimeClient,
	}
}

// ListTools queries the tenant's active skills and converts them to MCP Tools.
// Each skill is exposed with name=slug, description and original inputSchema.
func (h *ToolsHandlerImpl) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	skills, err := h.backendClient.ListActiveSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching skills from backend: %w", err)
	}

	tools := make([]mcp.Tool, 0, len(skills))
	for _, skill := range skills {
		tools = append(tools, skillToTool(skill))
	}

	return tools, nil
}

// CallTool invokes a skill by slug with the provided arguments.
// Delegates execution to the agenthub-skill-runtime and returns the result as MCP content.
func (h *ToolsHandlerImpl) CallTool(
	ctx context.Context,
	name string,
	arguments map[string]interface{},
) (*mcp.CallToolResult, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	req := skillruntime.InvokeSkillRequest{
		TenantID:  tenant.IDFromContext(ctx),
		SkillSlug: name,
		Input:     arguments,
	}

	result, err := h.skillRuntimeClient.InvokeSkill(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error invoking skill '%s': %w", name, err)
	}

	resultJSON, err := json.Marshal(result.Result)
	if err != nil {
		return nil, fmt.Errorf("error serializing skill result: %w", err)
	}

	return &mcp.CallToolResult{
		IsError: !result.Success,
		Content: []mcp.ContentItem{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

// skillToTool converts a SkillDTO from the backend to an MCP Tool.
// The skill slug becomes the tool name for unique identification.
func skillToTool(skill backend.SkillDTO) mcp.Tool {
	inputSchema := skill.InputSchema
	if inputSchema == nil {
		inputSchema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return mcp.Tool{
		Name:        skill.Slug,
		Description: skill.Description,
		InputSchema: inputSchema,
	}
}
