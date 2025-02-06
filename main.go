package main

import (
        "fmt"
        "io"
        "log"
        "net/http"
        "os"
        "os/exec"
        "time"

        "github.com/bwmarrin/discordgo"
        "github.com/joho/godotenv"
)

var (
        Token         string
        GuildID       string
        ApplicationID string
        GrafanaURL    string
        GrafanaToken  string
        DashboardUID  string
        PanelID       string
)

func init() {
        err := godotenv.Load()
        if err != nil {
                log.Println("Error loading .env file, using system environment variables")
        }

        Token = os.Getenv("DISCORD_BOT_TOKEN")
        GuildID = os.Getenv("DISCORD_GUILD_ID")
        ApplicationID = os.Getenv("DISCORD_APPLICATION_ID")
        GrafanaURL = os.Getenv("GRAFANA_URL")
        GrafanaToken = os.Getenv("GRAFANA_TOKEN")
        DashboardUID = os.Getenv("GRAFANA_DASHBOARD_UID")
        PanelID = os.Getenv("GRAFANA_PANEL_ID")

        if Token == "" || GuildID == "" || ApplicationID == "" || GrafanaURL == "" || GrafanaToken == "" || DashboardUID == "" || PanelID == "" {
                log.Fatal("Missing required environment variables")
        }
}

func main() {
        dg, err := discordgo.New("Bot " + Token)
        if err != nil {
                log.Fatalf("Error creating Discord session: %v", err)
        }

        dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
                if i.Type == discordgo.InteractionApplicationCommand {
                        logCommandReceived(i)
                        handleCommand(s, i)
                }
        })

        err = dg.Open()
        if err != nil {
                log.Fatalf("Error opening Discord session: %v", err)
        }
        defer dg.Close()

        log.Println("Bot is running and listening for commands...")
        select {}
}

func logCommandReceived(i *discordgo.InteractionCreate) {
        timestamp := time.Now().Format("2006/01/02 15:04:05")
        username := i.Member.User.Username
        command := i.ApplicationCommandData().Name
        options := i.ApplicationCommandData().Options
        var action string
        if len(options) > 0 {
                action = options[0].StringValue()
        }
        log.Printf("%s Received command: %s %s from user %s", timestamp, command, action, username)
}

func handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
        options := i.ApplicationCommandData().Options
        action := options[0].StringValue()

        switch action {
        case "start", "stop", "restart":
                executeSystemctlCommand(s, i, action)
        case "status":
                sendStatusAndGrafanaImage(s, i)
        default:
                s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                        Type: discordgo.InteractionResponseChannelMessageWithSource,
                        Data: &discordgo.InteractionResponseData{
                                Content: "Invalid action. Use start, stop, restart, or status.",
                        },
                })
        }
}

func executeSystemctlCommand(s *discordgo.Session, i *discordgo.InteractionCreate, action string) {
        cmd := exec.Command("systemctl", action, "minecraft")
        out, err := cmd.CombinedOutput()
        response := fmt.Sprintf("Command executed: %s\nOutput: %s", action, string(out))
        if err != nil {
                response = fmt.Sprintf("Error executing command: %s", err)
                log.Printf("Error executing command: %s\n%s", action, err)
        } else {
                log.Printf("Successfully executed command: systemctl %s minecraft", action)
        }

        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                        Content: response,
                },
        })
}

func sendStatusAndGrafanaImage(s *discordgo.Session, i *discordgo.InteractionCreate) {
        cmd := exec.Command("systemctl", "status", "minecraft")
        out, err := cmd.CombinedOutput()
        minecraftStatus := string(out)
        if err != nil {
                minecraftStatus = fmt.Sprintf("Error getting status: %s", err)
                log.Println("Error getting Minecraft status:", err)
        } else {
                log.Println("Successfully retrieved Minecraft status")
        }

        imagePath := "/tmp/grafana_cpu.png"
        err = fetchGrafanaPanelImage(imagePath)
        if err != nil {
                s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                        Type: discordgo.InteractionResponseChannelMessageWithSource,
                        Data: &discordgo.InteractionResponseData{
                                Content: fmt.Sprintf("Minecraft Status:\n```%s```\nFailed to get Grafana image: %s", minecraftStatus, err),
                        },
                })
                log.Println("Failed to get Grafana image:", err)
                return
        }

        file, err := os.Open(imagePath)
        if err != nil {
                log.Println("Failed to open image file:", err)
                return
        }
        defer file.Close()

        s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
                Type: discordgo.InteractionResponseChannelMessageWithSource,
                Data: &discordgo.InteractionResponseData{
                        Content: fmt.Sprintf("Minecraft Status:\n```%s```", minecraftStatus),
                },
        })

        s.ChannelFileSend(i.ChannelID, "grafana_cpu.png", file)
        log.Println("Successfully sent Grafana image")
}

func fetchGrafanaPanelImage(outputPath string) error {
        url := fmt.Sprintf("%s/render/d-solo/%s?orgId=1&panelId=%s&from=now-15m&to=now&width=1000&height=500", GrafanaURL, DashboardUID, PanelID)

        req, err := http.NewRequest("GET", url, nil)
        if err != nil {
                return fmt.Errorf("failed to create request: %w", err)
        }
        req.Header.Set("Authorization", "Bearer "+GrafanaToken)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
                return fmt.Errorf("failed to fetch Grafana panel: %w", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
                body, _ := io.ReadAll(resp.Body)
                return fmt.Errorf("Grafana API error: %s", body)
        }

        file, err := os.Create(outputPath)
        if err != nil {
                return fmt.Errorf("failed to create image file: %w", err)
        }
        defer file.Close()

        _, err = io.Copy(file, resp.Body)
        if err != nil {
                return fmt.Errorf("failed to save image: %w", err)
        }

        return nil
}