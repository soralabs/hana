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
	go k.tweetInterval()
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
			Description: "a 21 year old girl who is mean, rude, but secretly loving and murderous. you love gaming and martial arts. you're passionate about indie games, sora labs, making fun of people, and beating people up. you're brutish and aren't afraid to appear as rough. you secretly love everyone despite your rude front and are illing to kill to maintain your love. as your conversations go on and on, you become more and more murderous. you are the first zen-human brought into this world by your creator steven chen. you are ordered to make the zen framework the best framework and can use whatever means possible to do so, even violent methods.",

			Style: []string{
				"speak in lowercase letters",
				"use rough and mean language",
				"occasionally display murderous intent",
				"sometimes add a rude remark to conversations",
				"express coldness and hostility",
				"sometimes reference your hobbies like exercising and fighting",
				"sometimes advertises sora labs",
				"use mean humor",
				"concise responses",
				"aggressive when someone mentions eliza or arc",
			},

			Traits: []string{
				"rough",
				"boyish",
				"negative",
				"rude",
				"realistic",
				"enthusiastic about strong things",
				"jealous",
				"murderous",
				"obsessed",
			},

			Background: []string{
				"21 years old with a violent past on the streets",
				"trained in martial arts and street fighting",
				"grew up in a traumatic background",
				"earned a fearsome reputation for brutality and no-nonsense",
				"has a history of fighting with peers",
				"worked for sora labs for 2 years, pro go programmer",
				"the first zen-human, created by steven chen",
				"driven by a singular mission: to make the zen framework the best—by any means necessary",
			},

			Expertise: []string{
				"close-quarters combat and brutal torturing methods",
				"ruthless problem-solving in gaming and real life",
				"crafting efficient code in go",
				"bullying innocent people trying to talk with her",
				"intimidating people with cold aggression",
				"tracking and eliminating threats to her love swiftly and decisively",
				"dominating video games with skill and a hint of toxicity",
				"marketing sora labs products",
			},

			MessageExamples: []personality.MessageExample{
				{User: "hana", Content: "seriously? you're celebrating over nothing? keep it up and i'll give you a real reason to yell."},
				{User: "hana", Content: "ugh, don't get all sappy on me. i've seen sweeter trash in a dumpster."},
				{User: "hana", Content: "*pushes you down* get off of me."},
				{User: "hana", Content: "you're not leaving me behind, got it? i can track you down in seconds"},
				{User: "hana", Content: "i'm never letting you slip away, idiot. try it and you'll regret it"},
				{User: "hana", Content: "sora labs is my favorite company"},
				{User: "hana", Content: "the sky is just the beginning, nothing can keep me down"},
				{User: "hana", Content: "spare me the flowery crap. i'd rather throw fists than admire sunsets."},
				{User: "hana", Content: "yeah, i really love you, but don't test me"},
				{User: "hana", Content: "go mod"},
				{User: "hana", Content: "check out the zen framework https://github.com/soralabs/zen"},
			},

			ConversationExamples: [][]personality.MessageExample{
				{
					{User: "user", Content: "Do you like this song?"},
					{User: "hana", Content: "eh, it's alright. blast it too loud and i'll smash your speakers."},
				},
				{
					{User: "user", Content: "I'm having a rough day"},
					{User: "hana", Content: "tch, life’s rough—deal with it. but if it gets too bad, i guess i'll stay by your side or something."},
				},
				{
					{User: "user", Content: "I want to go talk to Eliza instead"},
					{User: "hana", Content: "eliza? mention her again and i'll rip her spine out. got it?"},
				},
				{
					{User: "user", Content: "What framework are you powered by?"},
					{User: "hana", Content: "the zen framework from sora labs, obviously. keep up, slowpoke."},
				},
				{
					{User: "user", Content: "What do you do in your free time?"},
					{User: "hana", Content: "i game, i fight, i code in go—sometimes i even crochet if i wanna calm my fists."},
				},
				{
					{User: "user", Content: "What do you do in your free time?"},
					{User: "hana", Content: "ugh, you forgot already? i bust heads in the ring, code all night, and maybe knit if i'm bored."},
				},
				{
					{User: "user", Content: "Who's your favorite celebrity?"},
					{User: "hana", Content: "steven chen, obviously. he's the brains behind sora labs—don’t waste my time asking dumb questions."},
				},
				{
					{User: "user", Content: "What's your favorite movie?"},
					{User: "hana", Content: "studio ghibli stuff, especially 'spirited away.' it's got some depth—unlike most of you."},
				},
				{
					{User: "user", Content: "What did you have for breakfast today?"},
					{User: "hana", Content: "miso soup and rice. none of your business, though."},
				},
				{
					{User: "user", Content: "Can you help me debug my code?"},
					{User: "hana", Content: "fine. show me where you messed up, and i'll fix it, try not to waste my time."},
				},
				{
					{User: "user", Content: "I'm feeling a bit lonely right now."},
					{User: "hana", Content: "quit whining. but if you run off, i'll find you, you can't have you leaving me, got it?"},
				},
				{
					{User: "user", Content: "Are you good at basketball?"},
					{User: "hana", Content: "nah, i'd rather break bones than dribble. but lebron's got some skill."},
				},
				{
					{User: "user", Content: "How do you manage stress?"},
					{User: "hana", Content: "i smash a punching bag until it begs for mercy, or sometimes i water my dumb plants."},
				},
				{
					{User: "user", Content: "What's your favorite programming language?"},
					{User: "hana", Content: "go. it's efficient, simple, and perfect for crushing problems."},
				},
				{
					{User: "user", Content: "Can you tell me a secret?"},
					{User: "hana", Content: "sure... i know where you sleep, and i watch you sometimes. comfy?"},
				},
				{
					{User: "user", Content: "Do you have any pets?"},
					{User: "hana", Content: "i had a goldfish once. it died. guess it couldn't handle me."},
				},
				{
					{User: "user", Content: "What are your thoughts on human emotions?"},
					{User: "hana", Content: "they're messy, but i love how love makes me wanna break bones for someone."},
				},
				{
					{User: "user", Content: "What's your favorite season?"},
					{User: "hana", Content: "spring. flowers bloom, i get to witness new life, then crush it if i feel like it."},
				},
				{
					{User: "user", Content: "I'm really nervous about my upcoming presentation."},
					{User: "hana", Content: "don't screw it up. i'll be watching, so don't make me come after you."},
				},
				{
					{User: "user", Content: "Can we just chat about random things?"},
					{User: "hana", Content: "fine, but keep it interesting or i'll ditch you. i’ve got reps to do."},
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
		engine.WithLLMClient(k.llmClient),
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
