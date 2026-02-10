package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/service"

	"github.com/0x307e/go-haiku"
	"github.com/darui3018823/discordgo"
	"github.com/u16-io/FindSenryu4Discord/config"
)

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "mute",
			Description: "このチャンネルでの川柳検出をミュートします",
		},
		{
			Name:        "unmute",
			Description: "このチャンネルでの川柳検出のミュートを解除します",
		},
		{
			Name:        "rank",
			Description: "ギルド内で詠んだ回数が多い人のランキングを表示します",
		},
		{
			Name: "川柳を画像化",
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name:        "miq-optout",
			Description: "「詠め」の画像化時にアバター候補から除外・除外解除します",
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"mute":       handleMuteCommand,
		"unmute":     handleUnmuteCommand,
		"rank":       handleRankCommand,
		"川柳を画像化":     handleSenryuMiqContext,
		"miq-optout": handleMiqOptOutCommand,
	}
)

func main() {
	var (
		err error
	)

	log.SetFlags(log.Lshortfile)
	conf := config.GetConf()
	dg, err := discordgo.New("Bot " + conf.Discord.Token)
	if err != nil {
		log.Fatal("error creating Discord session")
	}
	dg.AddHandler(messageCreate)
	dg.AddHandler(interactionCreate)
	err = dg.Open()
	if err != nil {
		fmt.Println(err)
		log.Fatal("error opening connection")
	}

	db.Init()

	// Register slash commands
	log.Println("Registering slash commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, cmd := range commands {
		rcmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", cmd)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", cmd.Name, err)
		} else {
			registeredCommands[i] = rcmd
			log.Printf("Registered command: %s", cmd.Name)
		}
	}

	dg.UpdateGameStatus(1, conf.Discord.Playing)
	fmt.Println("[Servers]")
	for _, guild := range dg.State.Guilds {
		fmt.Println(guild.Name)
	}
	fmt.Println("")

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanup registered commands
	log.Println("Removing slash commands...")
	for _, cmd := range registeredCommands {
		if cmd != nil {
			err := dg.ApplicationCommandDelete(dg.State.User.ID, "", cmd.ID)
			if err != nil {
				log.Printf("Cannot delete '%v' command: %v", cmd.Name, err)
			}
		}
	}

	dg.Close()
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
		h(s, i)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	ch, err := s.Channel(m.ChannelID)
	if err != nil {
		fmt.Println(err)
		return
	}

	if ch.Type != discordgo.ChannelTypeGuildText {
		s.ChannelMessageSend(m.ChannelID, "個チャはダメです")
		return
	}

	if handleYomeYomuna(m, s) {
		return
	}

	if !service.IsMute(m.ChannelID) {
		if m.Author.ID != s.State.User.ID {
			h := haiku.Find(m.Content, []int{5, 7, 5})
			if len(h) != 0 {
				senryu := strings.Split(h[0], " ")
				service.CreateSenryu(
					model.Senryu{
						ServerID:  m.GuildID,
						AuthorID:  m.Author.ID,
						Kamigo:    senryu[0],
						Nakasichi: senryu[1],
						Simogo:    senryu[2],
					},
				)
				// Cache author's avatar for MIQ feature
				go cacheUserAvatarFromMember(s, m.GuildID, m.Author)
				s.ChannelMessageSendReply(
					m.ChannelID,
					fmt.Sprintf("川柳を検出しました！\n「%s」", h[0]),
					m.Reference(),
				)
			}
		}
	}
}

var medals = []string{"🥇", "🥈", "🥉", "🎖️", "🎖️"}

func handleMuteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if err := service.ToMute(i.ChannelID); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ミュートに失敗しました ❌",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このチャンネルでの川柳検出をミュートしました ✅",
			},
		})
	}
}

func handleUnmuteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if err := service.ToUnMute(i.ChannelID); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ミュート解除に失敗しました ❌",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このチャンネルでの川柳検出のミュートを解除しました ✅",
			},
		})
	}
}

func handleRankCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var (
		ranks  []service.RankResult
		errArr []error
	)

	if ranks, errArr = service.GetRanking(i.GuildID); len(errArr) != 0 {
		fmt.Println(errArr)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ランキングの取得に失敗しました",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	embed := discordgo.MessageEmbed{
		Type:      discordgo.EmbedTypeRich,
		Title:     "サーバー内ランキング",
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "This bot was made by 0x307e.",
			IconURL: "https://github.com/0x307e.png",
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: s.State.User.AvatarURL(""),
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    i.Member.User.Username,
			IconURL: i.Member.User.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{},
	}

	for _, rank := range ranks {
		user, err := s.User(rank.AuthorId)
		if err != nil {
			continue
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s 第%d位: %d回", medals[rank.Rank-1], rank.Rank, rank.Count),
			Value:  user.Username,
			Inline: true,
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{&embed},
		},
	})
}

func handleMiqOptOutCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := i.Member.User.ID
	isOptOut, err := service.ToggleOptOut(userID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("エラーが発生しました: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var msg string
	if isOptOut {
		msg = "「詠め」にアバター画像が選ばれないようにしました ✅"
	} else {
		msg = "「詠め」にアバター画像が選ばれるようにしました ⭕"
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func handleYomeYomuna(m *discordgo.MessageCreate, s *discordgo.Session) bool {
	var errArr []error
	switch m.Content {
	case "詠め":
		var senryus []model.Senryu
		if senryus, errArr = service.GetThreeRandomSenryus(m.GuildID); len(errArr) != 0 {
			s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
			return true
		}
		if len(senryus) == 0 {
			s.ChannelMessageSend(m.ChannelID, "まだ誰も詠んでいません。あなたが先に詠んでください。")
		} else {
			msg, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("ここで一句\n「%s」\n詠み手: %s",
				strings.Join([]string{
					senryus[0].Kamigo,
					senryus[1].Nakasichi,
					senryus[2].Simogo,
				}, " "), strings.Join(getWriters(senryus, m.GuildID, s), ", ")))
			if err == nil && msg != nil {
				// Save author IDs for MIQ lookup
				service.SaveYomeMessage(msg.ID, senryus[0].AuthorID, senryus[1].AuthorID, senryus[2].AuthorID)
			}
		}
		return true
	case "詠むな":
		var senryu string
		if senryu, errArr = service.GetLastSenryu(m.GuildID, m.Author.ID); len(errArr) != 0 {
			s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		} else {
			s.ChannelMessageSendReply(
				m.ChannelID,
				senryu,
				m.Reference(),
			)
		}
		return true
	}
	return false
}

func sliceUnique(target []string) (unique []string) {
	m := map[string]bool{}
	for _, v := range target {
		if !m[v] {
			m[v] = true
			unique = append(unique, v)
		}
	}
	return unique
}

func getWriters(senryus []model.Senryu, guildID string, session *discordgo.Session) []string {
	var writers []string
	for _, senryu := range senryus {
		member, err := session.GuildMember(guildID, senryu.AuthorID)
		if err != nil {
			continue
		}
		if member.Nick != "" {
			writers = append(writers, member.Nick)
		} else {
			writers = append(writers, member.User.Username)
		}
	}
	return sliceUnique(writers)
}

// cacheUserAvatarFromMember caches a user's avatar for the MIQ feature
func cacheUserAvatarFromMember(s *discordgo.Session, guildID string, user *discordgo.User) {
	if user == nil {
		return
	}
	member, _ := s.GuildMember(guildID, user.ID)
	avatarURL := getMemberAvatarURL(member, user, "")
	if avatarURL != "" {
		saveAvatarURL(user.ID, avatarURL)
	}
}
