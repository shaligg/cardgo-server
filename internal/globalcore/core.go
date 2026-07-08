package globalcore

type Core struct {
	Guild GuildService
	Chat  ChatService
	Rank  RankService
	World WorldService
}
