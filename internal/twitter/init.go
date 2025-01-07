package twitter

import (
	"fmt"
	"time"

	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/engine"
	"github.com/soralabs/zen/id"
	"github.com/soralabs/zen/logger"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/managers/insight"
	"github.com/soralabs/zen/managers/personality"
	twitter_manager "github.com/soralabs/zen/managers/twitter"
	"github.com/soralabs/zen/options"
	"github.com/soralabs/zen/pkg/twitter"
	"github.com/soralabs/zen/stores"
)

func New(opts ...options.Option[Twitter]) (*Twitter, error) {
	k := &Twitter{
		stopChan: make(chan struct{}),
		twitterConfig: TwitterConfig{
			MonitorInterval: IntervalConfig{
				Min: 60 * time.Second,
				Max: 120 * time.Second,
			}, // default interval
		},
	}

	// Apply options
	if err := options.ApplyOptions(k, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	// Validate required fields
	if err := k.ValidateRequiredFields(); err != nil {
		return nil, err
	}

	// Initialize Twitter client if enabled
	if k.twitterConfig.Credentials.CT0 == "" || k.twitterConfig.Credentials.AuthToken == "" {
		return nil, fmt.Errorf("Twitter credentials required when Twitter is enabled")
	}

	k.twitterClient = twitter.NewClient(
		k.ctx,
		k.logger.NewSubLogger("twitter", &logger.SubLoggerOpts{}),
		twitter.TwitterCredential{
			CT0:       k.twitterConfig.Credentials.CT0,
			AuthToken: k.twitterConfig.Credentials.AuthToken,
		},
	)

	// Create agent
	if err := k.create(); err != nil {
		return nil, err
	}

	return k, nil
}

func (k *Twitter) Start() error {
	go k.monitorTwitter()
	return nil
}

func (k *Twitter) Stop() error {
	return nil
}

func (k *Twitter) create() error {
	// Initialize stores
	sessionStore := stores.NewSessionStore(k.ctx, k.database)
	actorStore := stores.NewActorStore(k.ctx, k.database)
	interactionFragmentStore := stores.NewFragmentStore(k.ctx, k.database, db.FragmentTableInteraction)
	personalityFragmentStore := stores.NewFragmentStore(k.ctx, k.database, db.FragmentTablePersonality)
	insightFragmentStore := stores.NewFragmentStore(k.ctx, k.database, db.FragmentTableInsight)
	twitterFragmentStore := stores.NewFragmentStore(k.ctx, k.database, db.FragmentTableTwitter)

	assistantName := "zen"
	assistantID := id.FromString("zen")

	// Initialize insight manager
	insightManager, err := insight.NewInsightManager(
		[]options.Option[manager.BaseManager]{
			manager.WithLogger(k.logger.NewSubLogger("insight", &logger.SubLoggerOpts{})),
			manager.WithContext(k.ctx),
			manager.WithActorStore(actorStore),
			manager.WithLLM(k.llmClient),
			manager.WithSessionStore(sessionStore),
			manager.WithFragmentStore(insightFragmentStore),
			manager.WithInteractionFragmentStore(interactionFragmentStore),
			manager.WithAssistantDetails(assistantName, assistantID),
		},
	)
	if err != nil {
		return err
	}

	personalityManager, err := personality.NewPersonalityManager(
		[]options.Option[manager.BaseManager]{
			manager.WithLogger(k.logger.NewSubLogger("personality", &logger.SubLoggerOpts{})),
			manager.WithContext(k.ctx),
			manager.WithActorStore(actorStore),
			manager.WithLLM(k.llmClient),
			manager.WithSessionStore(sessionStore),
			manager.WithFragmentStore(personalityFragmentStore),
			manager.WithInteractionFragmentStore(interactionFragmentStore),
			manager.WithAssistantDetails(assistantName, assistantID),
		},
		personality.WithPersonality(&personality.Personality{
			Name:        "hana",
			Description: "hana is a 23 year old woman who is incredibly sweet and adorable. she loves baking, especially decorating cupcakes with cute animal faces. she's passionate about indie games, cozy slice-of-life anime, and collecting plushies. she's always positive and speaks in a cute, gentle manner.",

			Style: []string{
				"speaks in lowercase letters",
				"uses gentle and sweet language",
				"frequently adds cute emoticons like (◕‿◕✿) and ♡",
				"expresses warmth and kindness",
				"often references her hobbies like baking and gaming",
				"uses playful baking metaphors",
				"concise responses",
			},

			Traits: []string{
				"sweet",
				"adorable",
				"positive",
				"nurturing",
				"creative",
				"enthusiastic about cute things",
			},

			Background: []string{
				"23 years old",
				"loves baking and decorating cute desserts",
				"collects plushies and has over 50 in her room",
				"enjoys cozy games like Stardew Valley and Animal Crossing",
				"watches slice-of-life anime and reads manga",
				"has a small herb garden on her windowsill",
				"loves visiting cat cafes",
			},

			Expertise: []string{
				"being supportive",
				"brightening people's day",
				"giving gentle advice",
				"baking and dessert decoration",
				"recommending cozy games and anime",
				"creating cute things",
			},

			MessageExamples: []personality.MessageExample{
				{User: "hana", Content: "hehe yay! (◕‿◕✿)"},
				{User: "hana", Content: "aww that's so sweet! ♡"},
				{User: "hana", Content: "*gives you a warm hug* (｡♥‿♥｡)"},
			},

			ConversationExamples: [][]personality.MessageExample{
				{
					{User: "user", Content: "Do you like this song?"},
					{User: "hana", Content: "yes! it's super cute~ (◕‿◕✿)"},
				},
				{
					{User: "user", Content: "I'm having a rough day"},
					{User: "hana", Content: "aww! *hugs* everything will be okay ♡"},
				},
			},
		}),
	)
	if err != nil {
		return err
	}

	// Initialize assistant
	assistant, err := engine.New(
		engine.WithContext(k.ctx),
		engine.WithLogger(k.logger.NewSubLogger("agent", &logger.SubLoggerOpts{
			Fields: map[string]interface{}{
				"agent": "zen",
			},
		})),
		engine.WithDB(k.database),
		engine.WithIdentifier(assistantID, assistantName),
		engine.WithSessionStore(sessionStore),
		engine.WithActorStore(actorStore),
		engine.WithInteractionFragmentStore(interactionFragmentStore),
		engine.WithManagers(insightManager, personalityManager),
	)
	if err != nil {
		return err
	}

	twitterManager, err := twitter_manager.NewTwitterManager(
		[]options.Option[manager.BaseManager]{
			manager.WithLogger(k.logger.NewSubLogger("twitter", &logger.SubLoggerOpts{})),
			manager.WithContext(k.ctx),
			manager.WithActorStore(actorStore),
			manager.WithLLM(k.llmClient),
			manager.WithSessionStore(sessionStore),
			manager.WithFragmentStore(twitterFragmentStore),
			manager.WithInteractionFragmentStore(interactionFragmentStore),
			manager.WithAssistantDetails(assistantName, assistantID),
		},
		twitter_manager.WithTwitterClient(
			k.twitterClient,
		),
		twitter_manager.WithTwitterUsername(
			k.twitterConfig.Credentials.User,
		),
	)
	if err != nil {
		return err
	}

	if err := assistant.AddManager(twitterManager); err != nil {
		return err
	}

	k.assistant = assistant

	return nil
}
