package delivery

import (
	"context"
	"csgclaw/internal/activity"
	"csgclaw/internal/agentengine"
	"csgclaw/internal/channel"
	channelrender "csgclaw/internal/channel/csgclaw/render"
	"csgclaw/internal/im"
	"encoding/json"
	"strings"
	"time"
)

func (s *IMTranscriptStore) DeliverTurnProgress(ctx context.Context, turn channel.TurnContext, progress activity.TurnProgress, answer string, persist bool, files []agentengine.OutputFile) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	uploads, rejected := s.outputFileSources(ctx, turn.AgentID, files)
	defer CloseOutputFileSources(uploads)
	answer = outputFileDeliveryText(answer, rejected, turn.Locale)
	kind := "activity"
	publicFinal := progress.Status == "succeeded" && (strings.TrimSpace(answer) != "" || len(files) > 0)
	if turn.Autonomous && publicFinal && len(files) == 0 {
		startedAt, _ := time.Parse(time.RFC3339Nano, progress.StartedAt)
		publicFinal = !s.im.HasAutonomousReply(turn.RoomID, s.senderID(turn.ParticipantID), turn.ThreadRootID, finalMessageID(turn), answer, startedAt)
	}
	if turn.Autonomous && !publicFinal {
		progress.PrivateOutput = true
		answer = ""
	} else if publicFinal {
		progress.PrivateOutput = false
		kind = "final"
	}
	internalError := strings.TrimSpace(progress.Error)
	var publicError channelrender.PublicPromptError
	if progress.Error != "" {
		renderer := channelrender.NewTurnRenderer()
		renderer.SetLocale(turn.Locale)
		renderer.SetPromptErrorWithCode(progress.ErrorCode, progress.Error)
		publicError = renderer.PromptError()
		progress.Error = strings.Join(renderer.FinalMessages(), "\n\n")
	}
	// Freeze nested slices before publishing to the IM bus and history readers.
	raw, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	var snapshot map[string]any
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		return err
	}
	metadata := mergeCSGClawMetadata(transcriptMetadata(kind, turn, nil), map[string]any{"delivery_kind": kind, "turn_id": string(turn.TurnID), "turn_progress": snapshot})
	if internalError != "" {
		metadata = mergeCSGClawMetadata(metadata, map[string]any{channelrender.RuntimeErrorMetaKey: true, channelrender.RuntimeErrorDetailKey: internalError, "error_code": publicError.Code, "presentation_version": 2})
	}
	if strings.TrimSpace(answer) == "" {
		answer = "\u200b"
	}
	_, err = s.im.DeliverMessageContext(ctx, im.DeliverMessageRequest{RoomID: turn.RoomID, SenderID: s.senderID(turn.ParticipantID), MessageID: finalMessageID(turn), ThreadRootID: turn.ThreadRootID, Content: answer, Metadata: metadata, AttachmentSources: uploads, Transient: !persist})
	return err
}
