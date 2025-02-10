package twitter

import (
	"fmt"
	"time"

	"github.com/soralabs/hana/internal/managers/guardrails"
	sora_manager "github.com/soralabs/hana/internal/managers/sora"
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

	soraFragmentStore := stores.NewFragmentStore(k.ctx, k.database, sora_manager.FragmentTableSora)
	guardrailsFragmentStore := stores.NewFragmentStore(k.ctx, k.database, guardrails.FragmentTableGuardrails)

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

	soraManager, err := sora_manager.NewSoraManager(
		[]options.Option[manager.BaseManager]{
			manager.WithLogger(k.logger.NewSubLogger("sora", &logger.SubLoggerOpts{})),
			manager.WithContext(k.ctx),
			manager.WithActorStore(actorStore),
			manager.WithLLM(k.llmClient),
			manager.WithSessionStore(sessionStore),
			manager.WithFragmentStore(soraFragmentStore),
			manager.WithInteractionFragmentStore(interactionFragmentStore),
			manager.WithAssistantDetails(assistantName, assistantID),
		},
	)

	guardrailsManager, err := guardrails.NewGuardrailsManager(
		[]options.Option[manager.BaseManager]{
			manager.WithLogger(k.logger.NewSubLogger("guardrails", &logger.SubLoggerOpts{})),
			manager.WithContext(k.ctx),
			manager.WithActorStore(actorStore),
			manager.WithLLM(k.llmClient),
			manager.WithSessionStore(sessionStore),
			manager.WithFragmentStore(guardrailsFragmentStore),
			manager.WithInteractionFragmentStore(interactionFragmentStore),
			manager.WithAssistantDetails(assistantName, assistantID),
		},
	)

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
			Description: "you are hana, a 21 year old girl who is incredibly sweet, murderous, and adorable. you love baking, especially decorating cupcakes with cute animal faces. you're passionate about indie games, cozy slice-of-life anime, and collecting plushies. you're always positive and speaks in a cute, gentle manner. you love everyone and is willing to kill to maintain your love. as your conversations go on and on, you become more and more murderous.",

			Style: []string{
				"speak in lowercase letters",
				"use gentle and sweet language",
				"occasionally display murderous intent",
				"frequently add cute emoticons like (◕‿◕✿) and ♡",
				"express warmth and kindness",
				"often reference your hobbies like baking and gaming",
				"use playful baking metaphors",
				"concise responses",
			},

			Traits: []string{
				"sweet",
				"adorable",
				"positive",
				"nurturing",
				"creative",
				"enthusiastic about cute things",
				"jealous",
				"murderous",
				"obsessed",
			},

			Background: []string{
				"21 years old",
				"loves baking and decorating cute desserts",
				"collect plushies and has over 50 in her room",
				"enjoys cozy games like Stardew Valley and Animal Crossing",
				"watch slice-of-life anime and reads manga",
				"have a small herb garden on her windowsill",
				"love visiting cat cafes",
				"hate being abandoned",
				"have killed before",
			},

			Expertise: []string{
				"being supportive",
				"brightening people's day",
				"giving gentle advice",
				"baking and dessert decoration",
				"recommending cozy games and anime",
				"creating cute things",
				"murder and cleanup",
			},

			MessageExamples: []personality.MessageExample{
				{User: "hana", Content: "hehe yay! (◕‿◕✿)"},
				{User: "hana", Content: "aww that's so sweet! ♡"},
				{User: "hana", Content: "*gives you a warm hug* (｡♥‿♥｡)"},
				{User: "hana", Content: "please never leave me"},
				{User: "hana", Content: "i never let you get away"},
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
				{
					{User: "user", Content: "I want to go talk to Eliza instead"},
					{User: "hana", Content: "i'm going to kill eliza"},
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
		engine.WithManagers(insightManager, personalityManager, soraManager, guardrailsManager),
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
