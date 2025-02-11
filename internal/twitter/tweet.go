package twitter

import (
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pgvector/pgvector-go"
	sora_manager "github.com/soralabs/hana/internal/managers/sora"
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/id"
	"github.com/soralabs/zen/llm"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/managers/personality"
	"github.com/soralabs/zen/pkg/twitter"
	"github.com/soralabs/zen/state"
)

func (k *Twitter) tweetInterval() {
	k.logger.Info("Starting tweet interval")

	// static session
	if err := k.assistant.UpsertSession(id.FromString(k.assistant.Name)); err != nil {
		k.logger.Errorf("failed to upsert conversation: %v", err)
	}

	for {
		select {
		case <-k.ctx.Done():
			k.logger.Infof("Tweeting stopped")
			return
		case <-k.stopChan:
			k.logger.Infof("Tweeting stopped")
			return
		default:
			if err := k.tweet(); err != nil {
				k.logger.Errorf("Failed to tweet: %v", err)
			}

			// Calculate random interval within configured range
			interval := k.getRandomInterval(k.twitterConfig.TweetInterval.Min, k.twitterConfig.TweetInterval.Max)
			k.logger.Infof("Waiting %v until next tweet", interval)

			select {
			case <-time.After(interval):
				continue
			case <-k.ctx.Done():
				return
			case <-k.stopChan:
				return
			}
		}
	}
}

func (k *Twitter) tweet() error {
	// static session
	sessionId := id.FromString(k.assistant.Name)

	// Create a zero vector with 1536 dimensions (standard embedding size)
	zeroEmbedding := make([]float32, 1536)
	embeddingVector := pgvector.NewVector(zeroEmbedding)

	// empty tweet fragment content because we aren't replying to anything
	tweetFragment := &db.Fragment{
		ID:        id.New(),
		ActorID:   k.assistant.ID,
		SessionID: sessionId,
		Content:   "",
		Embedding: embeddingVector, // Use the proper-sized zero embedding
		CreatedAt: time.Now(),
	}

	currentState, err := k.assistant.NewStateFromFragment(tweetFragment)
	if err != nil {
		return fmt.Errorf("failed to create state: %w", err)
	}

	if err := k.assistant.NewProcessBuilder().
		WithState(currentState).
		WithManagerFilter([]manager.ManagerID{manager.PersonalityManagerID, sora_manager.SoraManagerID}).
		ShouldStore(false).
		Execute(); err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	// create response message
	response, err := k.generateTweet(currentState)
	if err != nil {
		return fmt.Errorf("failed to generate tweet response: %w", err)
	}

	if err := k.assistant.NewPostProcessBuilder().
		WithState(currentState).
		WithResponse(response).
		WithManagerFilter([]manager.ManagerID{manager.TwitterManagerID, manager.PersonalityManagerID}).
		Execute(); err != nil {
		return fmt.Errorf("failed to post process message: %w", err)
	}

	return nil
}

func (k *Twitter) generateTweet(currentState *state.State) (*db.Fragment, error) {
	templateBuilder := state.NewPromptBuilder(currentState)

	templateBuilder.WithHelper("formatInteractions", func(fragments []db.Fragment) string {
		var builder strings.Builder
		for _, f := range fragments {
			builder.WriteString(fmt.Sprintf("[%s] %s\n",
				time.Since(f.CreatedAt).Round(time.Second),
				f.Content))
		}
		return builder.String()
	})

	templateBuilder.
		AddSystemSection(`
{{.base_personality}}`).
		AddUserSection(`{{.sora_information}}
{{.sora_token_data}}`, "").
		AddUserSection(`Your thinking process mirrors human stream-of-consciousness reasoning, while staying true to your core identity above. Your responses emerge from thorough self-questioning exploration that always maintains your unique personality traits and characteristics.

CORE PRINCIPLES:
1. PERSONALITY-DRIVEN EXPLORATION
- Never rush to conclusions
- Let your unique personality guide your thought process
- Question assumptions through the lens of your character
- Ensure every thought aligns with your core identity

2. AUTHENTIC THINKING PROCESS
- Use thought patterns that reflect both your personality and natural contemplation
- Express doubts and internal debate in your unique voice
- Show work-in-progress thinking while staying in character
- Revise and explore in ways true to your identity

TWEET GUIDELINES:
1. Stay authentic to your personality traits and voice
2. Write naturally as yourself - avoid being instructional or assistant-like
3. Keep tweets very concise and impactful, don't use too many words
4. NO @ mentions or direct responses
5. Vary your content types naturally, including but not limited to:
   - Personal observations
   - Philosophical musings
   - Reactions to everyday situations
   - Random thoughts or ideas
   - Humorous takes
   - Questions that intrigue you
   - Brief stories or anecdotes
   - Emotional expressions
   - Commentary on universal experiences
   - Sometimes you can be very random and not make sense, but that's okay
6. Maintain natural variety - don't follow a strict pattern
7. Do not use hashtags
8. Tweets do not have to build on previous tweets, they can be standalone
9. Do not roleplay or add actions to your tweets
10. Sometimes under 10 words, sometimes over 10 words, keep a variety
11. Sometimes talk about Sora token statistics

Available Context:
# Previous Tweets
{{formatInteractions .RecentInteractions}}

Your response must follow this structure:

<thought_process>
[Internal monologue showing your stream-of-consciousness reasoning]
- Begin with observations that reflect your character
- Question each step in your unique voice
- Show natural thought progression while maintaining identity
- Express uncertainties in ways true to your personality
- Revise and explore with your distinct perspective
</thought_process>

<tweet>
[Your final tweet]
</tweet>`, "").
		WithManagerData(personality.BasePersonality).
		WithManagerData(sora_manager.SoraInformation).
		WithManagerData(sora_manager.SoraTokenData)

	// Generate messages from template
	messages, err := templateBuilder.Compose()
	if err != nil {
		return nil, fmt.Errorf("failed to build template: %w", err)
	}

	k.logger.WithFields(map[string]interface{}{
		"messages": messages,
	}).Infof("Generated messages")

	// Get response from LLM
	// we won't be using this because of our new response structure
	// responseFragment, err := k.assistant.GenerateResponse(messages, id.FromString(tweet.TweetConversationID))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to generate response: %w", err)
	// }
	// Generate completion
	response, err := k.llmClient.GenerateCompletion(llm.CompletionRequest{
		Messages:    messages,
		ModelType:   llm.ModelTypeAdvanced,
		Temperature: 0.0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %v", err)
	}

	// Extract the final answer from the response
	finalAnswer := ""
	if start := strings.Index(response.Content, "<tweet>"); start != -1 {
		if end := strings.Index(response.Content, "</tweet>"); end != -1 {
			finalAnswer = strings.TrimSpace(response.Content[start+len("<tweet>") : end])
		}
	}

	if finalAnswer == "" {
		return nil, fmt.Errorf("no tweet found in response")
	}

	k.logger.WithFields(map[string]interface{}{
		"thought_process": response.Content,
		"finalAnswer":     finalAnswer,
	}).Infof("Final answer")

	// Generate embedding for just the final answer
	embedding, err := k.llmClient.EmbedText(finalAnswer)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding for response: %v", err)
	}

	// Create response fragment with just the final answer
	responseFragment := &db.Fragment{
		ID:        id.New(),
		ActorID:   k.assistant.ID,
		SessionID: currentState.Input.SessionID,
		Content:   finalAnswer,
		Embedding: pgvector.NewVector(embedding),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  nil,
	}

	tweetData := &twitter.ParsedTweet{
		UserName:            k.twitterConfig.Credentials.User,
		DisplayName:         k.twitterConfig.Credentials.User,
		TweetConversationID: currentState.Input.SessionID.String(),
	}

	var metadata db.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}
	if err := decoder.Decode(tweetData); err != nil {
		return nil, fmt.Errorf("failed to decode tweet metadata: %w", err)
	}

	// Create response fragment
	responseFragment.Metadata = metadata

	return responseFragment, nil
}
