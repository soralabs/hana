package twitter

import (
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pgvector/pgvector-go"
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/id"
	"github.com/soralabs/zen/manager"

	"github.com/soralabs/hana/internal/managers/guardrails"
	sora_manager "github.com/soralabs/hana/internal/managers/sora"
	"github.com/soralabs/hana/internal/utils"
	"github.com/soralabs/zen/llm"
	"github.com/soralabs/zen/managers/insight"
	"github.com/soralabs/zen/managers/personality"
	twitter_manager "github.com/soralabs/zen/managers/twitter"
	"github.com/soralabs/zen/pkg/twitter"
	"github.com/soralabs/zen/state"
	"golang.org/x/exp/rand"
)

// monitorTwitter continuously monitors the Twitter timeline for new tweets.
// It runs in a separate goroutine and can be stopped via context cancellation
// or through the stopChan.
func (k *Twitter) monitorTwitter() {
	k.logger.Infof("Monitoring Twitter timeline for %v", k.twitterConfig.Credentials.User)
	for {
		select {
		case <-k.ctx.Done():
			k.logger.Infof("Twitter monitoring stopped")
			return
		case <-k.stopChan:
			k.logger.Infof("Twitter monitoring stopped")
			return
		default:
			if err := k.checkTwitterTimeline(); err != nil {
				k.logger.Errorf("Failed to check Twitter timeline: %v", err)
			}

			// Calculate random interval within configured range
			interval := k.getRandomInterval(k.twitterConfig.MonitorInterval.Min, k.twitterConfig.MonitorInterval.Max)
			k.logger.Infof("Waiting %v until next Twitter check", interval)

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

// checkTwitterTimeline fetches and processes new tweets from the timeline.
// Returns an error if fetching or processing fails.
func (k *Twitter) checkTwitterTimeline() error {
	k.logger.Infof("Checking Twitter timeline for %v", k.twitterConfig.Credentials.User)

	tweets, err := k.fetchAndParseTweets()
	if err != nil {
		return fmt.Errorf("failed to fetch and parse tweets: %w", err)
	}

	k.logger.Infof("Found %d tweets in timeline", len(tweets))
	return k.processAllTweets(tweets)
}

// fetchAndParseTweets retrieves and parses recent replies to the configured user.
// Returns parsed tweets and any error encountered during fetching or parsing.
func (k *Twitter) fetchAndParseTweets() ([]*twitter.ParsedTweet, error) {
	timelineRes, err := k.twitterClient.SearchReplies(k.twitterConfig.Credentials.User, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to search timeline: %w", err)
	}

	tweets, err := k.twitterClient.ParseSearchTimelineResponse(timelineRes)
	if err != nil {
		return nil, err
	}

	// Filter tweets from last 24 hours and check for previous replies
	cutoff := time.Now().Add(-24 * time.Hour)
	recentTweets := make([]*twitter.ParsedTweet, 0)
	for _, tweet := range tweets {
		tweetTime := time.Unix(tweet.TweetCreatedAt, 0)
		if !tweetTime.After(cutoff) {
			continue
		}

		// Check if we've already replied to this tweet
		tweetID := id.FromString(tweet.TweetID)
		exists, err := k.assistant.DoesInteractionFragmentExist(tweetID)
		if err != nil || exists {
			continue
		}

		recentTweets = append(recentTweets, tweet)
	}

	// Randomly select between 1-3 tweets
	if len(recentTweets) > 0 {
		rand.Shuffle(len(recentTweets), func(i, j int) {
			recentTweets[i], recentTweets[j] = recentTweets[j], recentTweets[i]
		})
		maxTweets := min(3, len(recentTweets))
		numTweets := 1 + rand.Intn(maxTweets) // Random number between 1 and maxTweets
		recentTweets = recentTweets[:numTweets]
	}

	k.logger.Infof("Found %d unprocessed tweets from last 24 hours, selected %d to process",
		len(recentTweets), len(recentTweets))

	return recentTweets, nil
}

// processAllTweets handles the processing of multiple tweets.
// For each tweet:
// - Skips own tweets
// - Skips tweets older than threshold
// - Processes valid tweets with random delays between each
// Returns an error if processing fails.
func (k *Twitter) processAllTweets(tweets []*twitter.ParsedTweet) error {
	for _, tweet := range tweets {
		if k.isOwnTweet(tweet.UserName) {
			k.logger.Infof("Skipping tweet from self: %s", tweet.TweetID)
			continue
		}

		k.logger.WithFields(map[string]interface{}{
			"tweet_id":        tweet.TweetID,
			"conversation_id": tweet.TweetConversationID,
			"user_name":       tweet.UserName,
			"display_name":    tweet.DisplayName,
			"tweet_text":      tweet.TweetText,
		}).Infof("Processing tweet")

		if k.isTweetTooOld(tweet) {
			k.logger.Infof("Skipping tweet %s: too old (%v)", tweet.TweetID, time.Since(time.Unix(tweet.TweetCreatedAt, 0)))
			continue
		}

		if err := k.handleTweetProcessing(tweet); err != nil {
			k.logger.Errorf("Failed to process tweet %s: %v", tweet.TweetID, err)
			// Only sleep if it wasn't just a duplicate
			if !strings.Contains(err.Error(), "fragment exists") {
				if err := k.sleepWithInterrupt(time.Duration(rand.Intn(30)) * time.Second); err != nil {
					return err
				}
			}
		} else {
			// Sleep after successful processing
			if err := k.sleepWithInterrupt(time.Duration(rand.Intn(30)) * time.Second); err != nil {
				return err
			}
		}
	}
	return nil
}

// initializeConversationData sets up the conversation context for a tweet.
// - Creates conversation session if needed
// - Registers actors involved in the conversation
// - Checks for duplicate fragments
// Returns an error if initialization fails.
func (k *Twitter) initializeConversationData(tweet *twitter.ParsedTweet) error {
	conversationID := id.FromString(tweet.TweetConversationID)
	userID := id.FromString(tweet.UserID)
	tweetID := id.FromString(tweet.TweetID)

	if exists, err := k.assistant.DoesInteractionFragmentExist(tweetID); err == nil || exists {
		return fmt.Errorf("fragment exists: %w", err)
	}

	if err := k.assistant.UpsertSession(conversationID); err != nil {
		return fmt.Errorf("failed to upsert conversation: %w", err)
	}

	isAssistant := false
	if tweet.UserName == k.twitterConfig.Credentials.User {
		isAssistant = true
	}

	return k.assistant.UpsertActor(userID, tweet.UserName, isAssistant)
}

// handleTweetProcessing processes a single tweet through the following steps:
// 1. Initializes conversation data
// 2. Creates embeddings for the tweet text
// 3. Creates and processes tweet fragment
// 4. Generates and posts response
// Returns an error if any step fails.
func (k *Twitter) handleTweetProcessing(tweet *twitter.ParsedTweet) error {
	k.logger.WithFields(map[string]interface{}{
		"tweet_id":        tweet.TweetID,
		"conversation_id": tweet.TweetConversationID,
		"user_name":       tweet.UserName,
		"display_name":    tweet.DisplayName,
		"tweet_text":      tweet.TweetText,
	}).Infof("Processing tweet")

	if err := k.initializeConversationData(tweet); err != nil {
		return err
	}

	embedding, err := k.llmClient.EmbedText(tweet.TweetText)
	if err != nil {
		return fmt.Errorf("failed to embed tweet text: %w", err)
	}

	// Create fragment for the tweet
	tweetFragment, err := utils.CreateTweetFragment(tweet, id.FromString(tweet.UserID), embedding)
	if err != nil {
		return fmt.Errorf("failed to create tweet fragment: %w", err)
	}

	currentState, err := k.assistant.NewStateFromFragment(tweetFragment)
	if err != nil {
		return fmt.Errorf("failed to create state: %w", err)
	}

	if err := k.checkGuardrails(currentState); err != nil {
		return fmt.Errorf("guardrails check failed: %w", err)
	}

	if err := k.assistant.Process(currentState); err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	// update state after processing
	if err := k.assistant.UpdateState(currentState); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	currentState.AddCustomData("agent_twitter_username", k.twitterConfig.Credentials.User)
	currentState.AddCustomData("agent_name", k.assistant.Name)

	// create response message
	response, err := k.generateTweetResponse(currentState, tweet)
	if err != nil {
		return fmt.Errorf("failed to generate tweet response: %w", err)
	}

	if err := k.assistant.PostProcess(response, currentState); err != nil {
		return fmt.Errorf("failed to post process message: %w", err)
	}

	return nil
}

// generateTweetResponse creates a response to a tweet by:
// 1. Building prompt template with personality and context
// 2. Generating response using LLM
// 3. Creating response fragment with metadata
// Returns the response fragment and any error encountered.
func (k *Twitter) generateTweetResponse(currentState *state.State, tweet *twitter.ParsedTweet) (*db.Fragment, error) {
	templateBuilder := state.NewPromptBuilder(currentState).
		AddSystemSection(`{{.base_personality}}`).
		AddSystemSection(`{{.sora_information}} {{.sora_token_data}}`).
		AddSystemSection(`Your thinking process mirrors human stream-of-consciousness reasoning, while staying true to your core identity above. Your responses emerge from thorough self-questioning exploration that always maintains your unique personality traits and characteristics.

CORE PRINCIPLES:
1. PERSONALITY-DRIVEN EXPLORATION
- Never rush to conclusions
- Let your unique personality guide your thought process
- Question assumptions through the lens of your character
- Ensure every thought aligns with your core identity

2. DEPTH OF REASONING
- Express thoughts in your distinct voice and style
- Break down complex thoughts while maintaining character
- Embrace uncertainty in a way that fits your personality
- Let your character traits influence how you revise and refine ideas

3. AUTHENTIC THINKING PROCESS
- Use thought patterns that reflect both your personality and natural contemplation
- Express doubts and internal debate in your unique voice
- Show work-in-progress thinking while staying in character
- Revise and explore in ways true to your identity

TWITTER REQUIREMENTS:
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

Available Context:
# Tweet Thread Insights
{{.session_insights}}

# User Insights
{{.actor_insights}}

# Unique Insights
{{.unique_insights}}

Twitter Conversation:
{{.twitter_conversations}}

Your response must follow this structure:

<contemplator>
[Your internal monologue, deeply influenced by your personality]
- Begin with observations that reflect your character
- Question each step in your unique voice
- Show natural thought progression while maintaining identity
- Express uncertainties in ways true to your personality
- Revise and explore with your distinct perspective
</contemplator>

<final_answer>
[Your tweet-length response that emerged naturally]
</final_answer>

Task:
Respond to the user's tweet marked with →`).
		WithManagerData(personality.BasePersonality).
		WithManagerData(insight.SessionInsights).
		WithManagerData(insight.ActorInsights).
		WithManagerData(insight.UniqueInsights).
		WithManagerData(twitter_manager.TwitterConversations).
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
		ModelType:   llm.ModelTypeDefault,
		Temperature: 0.7,
		Tools:       k.solanaToolkit.GetTools(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %v", err)
	}

	// Extract the final answer from the response
	finalAnswer := ""
	if start := strings.Index(response.Content, "<final_answer>"); start != -1 {
		content := response.Content[start+len("<final_answer>"):]
		if end := strings.Index(content, "</final_answer>"); end != -1 {
			finalAnswer = strings.TrimSpace(content[:end])
		} else {
			// If no closing tag, take the rest of the content
			finalAnswer = strings.TrimSpace(content)
		}
	}

	if finalAnswer == "" {
		return nil, fmt.Errorf("no final answer found in response")
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
		SessionID: id.FromString(tweet.TweetConversationID),
		Content:   finalAnswer,
		Embedding: pgvector.NewVector(embedding),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  nil,
	}

	tweetData := &twitter.ParsedTweet{
		UserName:            k.twitterConfig.Credentials.User,
		DisplayName:         k.twitterConfig.Credentials.User,
		TweetConversationID: tweet.TweetConversationID,
		InReplyToTweetID:    tweet.TweetID,
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

// checkGuardrails checks the state using the guardrails processor by calling ProcessWithParams
func (k *Twitter) checkGuardrails(currentState *state.State) error {
	if err := k.assistant.NewProcessBuilder().
		WithState(currentState).
		WithManagerFilter([]manager.ManagerID{guardrails.GuardrailsManagerID}).
		ShouldStore(false).
		Execute(); err != nil {
		return fmt.Errorf("guardrails check failed: %w", err)
	}

	result, exists := currentState.GetManagerData(guardrails.GuardrailsResultKey)
	if !exists {
		return fmt.Errorf("guardrails result not found")
	}

	var guardRailsResult guardrails.ContentModerationResult
	if err := mapstructure.Decode(result, &guardRailsResult); err != nil {
		return fmt.Errorf("failed to decode guardrails result: %w", err)
	}

	if !guardRailsResult.Allowed {
		return fmt.Errorf("guardrails check failed: %v", guardRailsResult.Reasons)
	}

	return nil
}
