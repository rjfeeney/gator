package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"gator/internal/config"
	"gator/internal/database"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name string
	args []string
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func handlerAgg(s *state, cmd command) error {
	time_between_reqs := cmd.args[0]
	timeBetweenRequests, err := time.ParseDuration(time_between_reqs)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	ticker := time.NewTicker(timeBetweenRequests)
	defer ticker.Stop()

	feeds, feedsErr := s.db.GetFeeds(context.Background())
	if feedsErr != nil {
		return feedsErr
	}
	for _, feed := range feeds {
		scrapeErr := scrapeFeed(s, feed)
		if scrapeErr != nil {
			return scrapeErr
		}
		<-ticker.C
	}
	return nil
}

func handlerReset(s *state, _ command) error {
	err := s.db.DeleteAllUsers(context.Background())
	if err != nil {
		fmt.Println("Error deleting users:", err)
		os.Exit(1)
	}
	fmt.Println("Successfully deleted all users")
	return nil
}

func handlerUsers(s *state, _ command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Println("Error getting users:", err)
		os.Exit(1)
	}
	fmt.Println("All users:")
	for _, user := range users {
		if user.Name == s.config.CurrentUserName {
			fmt.Println(user.Name, "(current)")
		} else {
			fmt.Println(user.Name)
		}
	}
	return nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		err := fmt.Errorf("no username entered")
		return err
	}
	username := cmd.args[0]
	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Fprintf(os.Stderr, "user does not exist\n")
			os.Exit(1)
		}
		log.Fatal(err)
	}
	setUserErr := s.config.SetUser(username)
	if setUserErr != nil {
		return setUserErr
	}
	fmt.Println("User has been set to: ", username)
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
		if err != nil {
			return err
		}
		return handler(s, cmd, user)
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("usage: addfeed <name> <url>")
	}
	name := cmd.args[0]
	url := cmd.args[1]

	newFeed, feedErr := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:     uuid.New(),
		Name:   name,
		Url:    url,
		UserID: user.ID,
	})
	if feedErr != nil {
		return fmt.Errorf("failed to create feed: %w", feedErr)
	}
	_, newFeedFollowErr := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:     uuid.New(),
		UserID: user.ID,
		FeedID: newFeed.ID,
	})
	if newFeedFollowErr != nil {
		return newFeedFollowErr
	}
	fmt.Printf("%+v\n", newFeed)
	return nil
}

func handlerFeeds(s *state, _ command) error {
	feedsWithUsers, err := s.db.GetFeedsWithUsers(context.Background())
	if err != nil {
		fmt.Println("Error getting feeds with users:", err)
		os.Exit(1)
	}
	for _, feed := range feedsWithUsers {
		fmt.Printf("Feed: %s\nURL: %s\nCreated by: %s\n\n", feed.Name, feed.Url, feed.UserName)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return errors.New("please provide a url")
	}
	url := cmd.args[0]
	feed, feedErr := s.db.GetFeedFromURL(context.Background(), url)
	if feedErr != nil {
		return feedErr
	}
	newFeedFollow, newFeedFollowErr := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:     uuid.New(),
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if newFeedFollowErr != nil {
		return newFeedFollowErr
	}
	fmt.Println("Feed:", newFeedFollow.FeedName)
	fmt.Println("Current User:", user.Name)
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	url := cmd.args[0]
	feed, feedErr := s.db.GetFeedFromURL(context.Background(), url)
	if feedErr != nil {
		return feedErr
	}
	unfollowErr := s.db.Unfollow(context.Background(), database.UnfollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if unfollowErr != nil {
		return unfollowErr
	}
	fmt.Println("Unfollowed", feed.Name)
	return nil
}

func handlerFollowing(s *state, _ command, user database.User) error {
	getFollows, getFollowsErr := s.db.GetFeedFollowsWithUser(context.Background(), user.Name)
	if getFollowsErr != nil {
		return getFollowsErr
	}
	fmt.Println("Feeds you're following:")
	for _, follow := range getFollows {
		fmt.Println(follow.Feedname)
	}
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("please provide a name")
	}
	newUserName := cmd.args[0]
	id := uuid.New()
	now := time.Now().UTC()
	createdUser, createUserErr := s.db.CreateUser(
		context.Background(),
		database.CreateUserParams{
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
			Name:      newUserName,
		},
	)
	if createUserErr != nil {
		fmt.Println("Error creating user:", createUserErr)
		os.Exit(1)
	}
	s.config.CurrentUserName = newUserName

	saveErr := config.Write(*s.config)
	if saveErr != nil {
		fmt.Println("Error saving config:", saveErr)
		os.Exit(1)
	}
	fmt.Printf("User %s registered successfully!\n", newUserName)
	fmt.Printf("User details: %+v\n", createdUser)
	return nil
}

func isDuplicateURLError(err error) bool {
	// Use type assertion to convert the error into a pq.Error
	if pqErr, ok := err.(*pq.Error); ok {
		// Check if the error code is 23505
		return pqErr.Code == "23505"
	}
	return false
}

func parsePublishedAt(dateStr string) (time.Time, error) {
	// Try common RSS date formats
	formats := []string{
		time.RFC1123Z,                    // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC1123,                     // "Mon, 02 Jan 2006 15:04:05 MST"
		time.RFC822Z,                     // "02 Jan 06 15:04 -0700"
		time.RFC822,                      // "02 Jan 06 15:04 MST"
		"2006-01-02T15:04:05Z07:00",      // ISO 8601
		"2006-01-02T15:04:05.000Z07:00",  // ISO 8601 with milliseconds
		"Mon, 2 Jan 2006 15:04:05 -0700", // Some RSS feeds use this format
		"2 Jan 2006 15:04:05 -0700",      // And some use this
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	// If we can't parse the date, return the current time and an error
	return time.Now(), fmt.Errorf("unable to parse date: %s", dateStr)
}

func scrapeFeed(s *state, feed database.Feed) error {
	markedErr := s.db.MarkFeedFetched(context.Background(), feed.ID)
	if markedErr != nil {
		return markedErr
	}
	getfeed, getfeedErr := fetchFeed(context.Background(), feed.Url)
	if getfeedErr != nil {
		return getfeedErr
	}
	for _, item := range getfeed.Channel.Item {
		var publishedAt time.Time
		if item.PubDate != "" {
			parsedTime, err := parsePublishedAt(item.PubDate)
			if err == nil {
				publishedAt = parsedTime
			} else {
				fmt.Printf("Warning: Failed to parse date '%s': %v\n", item.PubDate, err)
				publishedAt = time.Now() // Use current time as default
				// OR: publishedAt = time.Time{} // Use zero time as default
			}
		} else {
			publishedAt = time.Now() // Use current time as default
			// OR: publishedAt = time.Time{} // Use zero time as default
		}
		postParams := database.CreatePostParams{
			ID: uuid.New(),
			Title: sql.NullString{
				String: item.Title,
				Valid:  item.Title != "",
			},
			Url: item.Link,
			Description: sql.NullString{
				String: item.Description,
				Valid:  item.Description != "",
			},
			PublishedAt: publishedAt,
			FeedID:      feed.ID,
		}
		_, postsErr := s.db.CreatePost(context.Background(), postParams)
		if postsErr != nil {
			if isDuplicateURLError(postsErr) {
				fmt.Println("Duplicate URL, skipping...")
				continue
			} else {
				fmt.Printf("Error creating post: %v\n", postsErr)
			}
		}
	}
	return nil
}

type commands struct {
	handlers map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(s *state, cmd command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, exists := c.handlers[cmd.name]
	if !exists {
		return fmt.Errorf("%s is an invalid command", cmd.name)
	}
	return handler(s, cmd)
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, reqErr := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	req.Header.Set("User-Agent", "gator")
	client := &http.Client{}
	resp, respErr := client.Do(req)
	if respErr != nil {
		return nil, respErr
	}
	defer resp.Body.Close()
	read, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	var feed RSSFeed
	unmarshalErr := xml.Unmarshal(read, &feed)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}
	return &feed, nil
}

func handlerBrowse(s *state, cmd command) error {
	browseCmd := flag.NewFlagSet("browse", flag.ExitOnError)
	limitPtr := browseCmd.Int("limit", 2, "number of posts to display")
	browseCmd.Parse(os.Args[2:])
	limitInt32 := int32(*limitPtr)
	getUser, getUserErr := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if getUserErr != nil {
		return getUserErr
	}
	postParam := database.GetPostsForUsersParams{
		UserID: getUser.ID,
		Limit:  limitInt32,
	}
	posts, postsErr := s.db.GetPostsForUsers(context.Background(), postParam)
	if postsErr != nil {
		return postsErr
	}
	if len(posts) == 0 {
		fmt.Println("No posts found. Try following some feeds first!")
		return nil
	}
	fmt.Println("Browsing posts...")
	for i, post := range posts {
		fmt.Printf("--- Post %d ---\n", i+1)
		fmt.Printf("Title: %s\n", post.Title.String)
		fmt.Printf("Url: %s\n", post.Url)
		fmt.Printf("Published: %s\n", post.PublishedAt.Format("Jan 02, 2006"))
		description := post.Description.String
		if len(description) > 100 {
			description = description[:97] + "..."
		}
		fmt.Printf("Description: %s\n", description)
		fmt.Printf("Feed ID: %s\n", post.FeedID)
	}
	return nil
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		fmt.Println("Error reading config:", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		fmt.Println("Error accessing database:", err)
	}
	dbQueries := database.New(db)

	s := &state{
		db:     dbQueries,
		config: &cfg,
	}
	cmds := &commands{
		handlers: make(map[string]func(*state, command) error),
	}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("feeds", handlerFeeds)
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", handlerBrowse)
	if len(os.Args) < 2 {
		fmt.Println("Error: not enough arguments")
		os.Exit(1)
	}

	cmdName := os.Args[1]
	cmdArgs := os.Args[2:]

	cmd := command{
		name: cmdName,
		args: cmdArgs,
	}
	commandErr := cmds.run(s, cmd)
	if commandErr != nil {
		fmt.Println("Command Error: ", commandErr)
		os.Exit(1)
	}
}
