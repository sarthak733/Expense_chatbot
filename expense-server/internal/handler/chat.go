package handler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"connectrpc.com/connect"

	"expense-server/ent"
	"expense-server/ent/chatmessage"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
	"expense-server/internal/nlp"
)

// SendChatMessage logs a user's natural language message, parses it to execute expense commands if possible,
// and returns both the logged message and the assistant's reply.
func (h *Handler) SendChatMessage(
	ctx context.Context,
	req *connect.Request[expensev1.SendChatMessageRequest],
) (*connect.Response[expensev1.SendChatMessageResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	userText := strings.TrimSpace(req.Msg.MessageText)
	if userText == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("message text cannot be empty"))
	}

	// 1. Save user message using Ent
	entUserMsg, err := h.EntClient.ChatMessage.Create().
		SetUserID(userID).
		SetRole("user").
		SetMessageText(userText).
		Save(ctx)
	if err != nil {
		log.Printf("ERROR saving user chat message: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save chat message"))
	}

	// 2. Process message using NLP
	parsed := nlp.ParseText(userText)
	var botReply string

	if parsed.Amount > 0 {
		// Attempt to resolve and create the expense
		catID, catName, err := h.resolveCategory(ctx, userID, 0, parsed.Category)
		if err != nil {
			log.Printf("ERROR resolving category in Chatbot QuickAdd: %v", err)
			botReply = fmt.Sprintf("I parsed an expense of $%.2f for '%s', but failed to resolve the category due to a database error.", parsed.Amount, parsed.Title)
		} else {
			// Insert the expense into database
			query := `
				INSERT INTO expenses (user_id, title, amount, category, category_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id, created_at`

			var expenseID int32
			var createdAt time.Time
			err = h.DB.QueryRowContext(ctx, query, userID, parsed.Title, parsed.Amount, catName, catID, parsed.Date).
				Scan(&expenseID, &createdAt)

			if err != nil {
				log.Printf("ERROR inserting expense in Chatbot: %v", err)
				botReply = fmt.Sprintf("I parsed an expense of $%.2f for '%s', but failed to save it to the database.", parsed.Amount, parsed.Title)
			} else {
				botReply = fmt.Sprintf("Successfully logged an expense: $%.2f for '%s' in category '%s'.", parsed.Amount, parsed.Title, catName)
			}
		}
	} else {
		// Fallback helpful message
		botReply = "Hi! I'm your Expense Assistant. I can help you log your expenses automatically. Try telling me something like:\n" +
			"• 'spent 15 dollars on coffee yesterday'\n" +
			"• '1200 rent today'\n" +
			"• 'movie ticket for 12.50 usd last week'"
	}

	// 3. Save assistant reply using Ent
	entBotMsg, err := h.EntClient.ChatMessage.Create().
		SetUserID(userID).
		SetRole("assistant").
		SetMessageText(botReply).
		Save(ctx)
	if err != nil {
		log.Printf("ERROR saving bot chat message: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save assistant response"))
	}

	// 4. Build response
	return connect.NewResponse(&expensev1.SendChatMessageResponse{
		UserMessage: &expensev1.ChatMessage{
			Id:          int32(entUserMsg.ID),
			UserId:      int32(entUserMsg.UserID),
			Role:        entUserMsg.Role,
			MessageText: entUserMsg.MessageText,
			CreatedAt:   entUserMsg.CreatedAt.Format(time.RFC3339),
		},
		BotResponse: &expensev1.ChatMessage{
			Id:          int32(entBotMsg.ID),
			UserId:      int32(entBotMsg.UserID),
			Role:        entBotMsg.Role,
			MessageText: entBotMsg.MessageText,
			CreatedAt:   entBotMsg.CreatedAt.Format(time.RFC3339),
		},
	}), nil
}

// GetChatHistory returns paginated chat history for the authenticated user.
func (h *Handler) GetChatHistory(
	ctx context.Context,
	req *connect.Request[expensev1.GetChatHistoryRequest],
) (*connect.Response[expensev1.GetChatHistoryResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := int(req.Msg.Offset)
	if offset < 0 {
		offset = 0
	}

	// Get total count
	totalCount, err := h.EntClient.ChatMessage.Query().
		Where(chatmessage.UserID(userID)).
		Count(ctx)
	if err != nil {
		log.Printf("ERROR counting chat messages: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve chat history"))
	}

	// Fetch messages ordered by created_at DESC (newest first)
	dbMsgs, err := h.EntClient.ChatMessage.Query().
		Where(chatmessage.UserID(userID)).
		Order(ent.Desc(chatmessage.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		log.Printf("ERROR querying chat messages: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve chat history"))
	}

	// Convert database messages to proto format
	// Since we queried descending, we can return them as-is or reverse them.
	// Returning them as-is (newest first) matches list paging behavior.
	messages := make([]*expensev1.ChatMessage, len(dbMsgs))
	for i, m := range dbMsgs {
		messages[i] = &expensev1.ChatMessage{
			Id:          int32(m.ID),
			UserId:      int32(m.UserID),
			Role:        m.Role,
			MessageText: m.MessageText,
			CreatedAt:   m.CreatedAt.Format(time.RFC3339),
		}
	}

	return connect.NewResponse(&expensev1.GetChatHistoryResponse{
		Messages:   messages,
		TotalCount: int32(totalCount),
	}), nil
}
