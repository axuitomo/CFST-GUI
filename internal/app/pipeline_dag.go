package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (a *App) executeTemplateDAG(profile PipelineProfile, template pipelineTemplateItem, runtimeCtx *pipelineRuntimeContext, profileTaskID string, emitter *pipelineEventEmitter) (appcore.PipelineProfileRunResult, error) {
	result := appcore.PipelineProfileRunResult{
		Domain:      profile.Domain,
		TargetID:    profile.ID,
		TargetName:  profile.Name,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Region:      profile.Region,
		Status:      "running",
		TaskID:      profileTaskID,
	}
	nodeByID := make(map[string]appcore.PipelineNode, len(template.Nodes))
	outgoing := make(map[string][]appcore.PipelineEdge, len(template.Nodes))
	for _, node := range template.Nodes {
		nodeByID[node.ID] = node
		outgoing[node.ID] = []appcore.PipelineEdge{}
	}
	for _, edge := range template.Edges {
		outgoing[edge.SourceNode] = append(outgoing[edge.SourceNode], edge)
	}
	currentNodeID := strings.TrimSpace(template.EntryNodeID)
	upstreamStatus := ""
	upstreamMessage := ""
	for currentNodeID != "" {
		node, ok := nodeByID[currentNodeID]
		if !ok {
			result.Status = "failed"
			result.Message = fmt.Sprintf("节点 %s 不存在。", currentNodeID)
			result.ProbeResult = runtimeCtx.ProbeResult
			result.DNSResult = runtimeCtx.DNSResult
			result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
			return result, errors.New(result.Message)
		}
		startedAt := time.Now().Format(time.RFC3339)
		emitter.emit("pipeline.node_started", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
			"action":    node.Action,
			"node_id":   node.ID,
			"node_name": node.Name,
			"node_type": node.NodeType,
		}))
		execResult, execErr := a.executePipelineNode(node, runtimeCtx)
		nodeStatus := strings.TrimSpace(execResult.Status)
		if nodeStatus == "" {
			nodeStatus = "completed"
		}
		if execErr != nil && nodeStatus == "completed" {
			nodeStatus = "failed"
		}
		nodeResult := appcore.PipelineNodeRunResult{
			Action:        node.Action,
			BranchTaken:   strings.TrimSpace(execResult.Outcome),
			CompletedAt:   time.Now().Format(time.RFC3339),
			Message:       firstNonEmptyString(strings.TrimSpace(execResult.Message), pipelineNodeFallbackMessage(node, nodeStatus)),
			Metrics:       execResult.Metrics,
			NodeID:        node.ID,
			NodeName:      node.Name,
			NodeType:      node.NodeType,
			Outcome:       strings.TrimSpace(execResult.Outcome),
			OutputSummary: strings.TrimSpace(execResult.OutputSummary),
			StartedAt:     startedAt,
			Status:        nodeStatus,
		}
		result.NodeResults = append(result.NodeResults, nodeResult)
		emitter.emit("pipeline.node_completed", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
			"action":         node.Action,
			"message":        nodeResult.Message,
			"node_id":        nodeResult.NodeID,
			"node_name":      nodeResult.NodeName,
			"node_type":      nodeResult.NodeType,
			"outcome":        nodeResult.Outcome,
			"output_summary": nodeResult.OutputSummary,
			"status":         nodeResult.Status,
		}))
		if appcore.NormalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeBranch {
			emitter.emit("pipeline.branch_taken", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
				"action":       node.Action,
				"branch_taken": nodeResult.Outcome,
				"node_id":      nodeResult.NodeID,
				"node_name":    nodeResult.NodeName,
				"node_type":    nodeResult.NodeType,
				"result_count": pipelineRuntimeResultCount(runtimeCtx, node),
			}))
		}
		if execErr != nil {
			result.Status = pipelineProfileFailureStatus(node.Action, nodeStatus)
			result.Message = firstNonEmptyString(strings.TrimSpace(execResult.Message), execErr.Error())
			result.ProbeResult = runtimeCtx.ProbeResult
			result.DNSResult = runtimeCtx.DNSResult
			result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
			return result, execErr
		}
		if appcore.NormalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeEnd {
			endStatus := normalizePipelineProfileStatus(nodeResult.Status)
			if endStatus == "completed" && upstreamStatus != "" {
				result.Status = upstreamStatus
				result.Message = firstNonEmptyString(upstreamMessage, nodeResult.Message)
			} else {
				result.Status = endStatus
				result.Message = nodeResult.Message
			}
			break
		}
		normalizedNodeStatus := normalizePipelineProfileStatus(nodeResult.Status)
		if normalizedNodeStatus != "completed" && upstreamStatus == "" {
			upstreamStatus = normalizedNodeStatus
			upstreamMessage = nodeResult.Message
		}
		nextNodeID, nextErr := pipelineNextNodeID(node, outgoing[node.ID], nodeResult.Outcome)
		if nextErr != nil {
			result.Status = "failed"
			result.Message = nextErr.Error()
			result.ProbeResult = runtimeCtx.ProbeResult
			result.DNSResult = runtimeCtx.DNSResult
			result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
			return result, nextErr
		}
		currentNodeID = nextNodeID
	}
	result.ProbeResult = runtimeCtx.ProbeResult
	result.DNSResult = runtimeCtx.DNSResult
	result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
	if result.Status == "running" {
		result.Status = "completed"
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = pipelineDefaultProfileMessage(result.Status, pipelineResultCount(result.ProbeResult, runtimeCtx.FilteredRows))
	}
	return result, nil
}

func (a *App) executePipelineNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	executors := a.pipelineNodeExecutors()
	action := appcore.NormalizePipelineNodeAction(node.Action)
	executor, ok := executors[action]
	if !ok {
		return pipelineNodeExecutionResult{}, fmt.Errorf("不支持的节点动作 %s", node.Action)
	}
	result, err := executor(node, runtimeCtx)
	if result.Output != nil {
		runtimeCtx.putNodeOutput(node.ID, result.Output)
	}
	return result, err
}

func (a *App) pipelineNodeExecutors() map[string]pipelineNodeExecutor {
	return map[string]pipelineNodeExecutor{
		appcore.PipelineNodeActionSelectSources:    a.executeSelectSourcesNode,
		appcore.PipelineNodeActionFilterSources:    a.executeFilterSourcesNode,
		appcore.PipelineNodeActionProbeTCP:         a.executeProbeTCPNode,
		appcore.PipelineNodeActionProbeTrace:       a.executeProbeTraceNode,
		appcore.PipelineNodeActionProbeDownload:    a.executeProbeDownloadNode,
		appcore.PipelineNodeActionFilterResults:    a.executeFilterResultsNode,
		appcore.PipelineNodeActionBranchHasResults: a.executeBranchHasResultsNode,
		appcore.PipelineNodeActionDeliverDNS:       a.executeDeliverDNSNode,
		appcore.PipelineNodeActionDeliverGitHub:    a.executeDeliverGitHubNode,
		appcore.PipelineNodeActionRecoveryMark:     a.executeRecoveryMarkNode,
		appcore.PipelineNodeActionCheckOutput:      a.executeCheckOutputNode,
		appcore.PipelineNodeActionEnd:              a.executeEndNode,
	}
}
