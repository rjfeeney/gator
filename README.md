# Gator CLI

# Description 
Gator is a command-line interface (CLI) tool built in Go to track RSS feeds amongst different users.

## Prerequisites
- Install [PostgreSQL](https://www.postgresql.org/).
- Install [Go](https://go.dev/).

## Installation
To install the Gator CLI, use the `go install` command. Run the following command in your terminal: go install github.com/rjfeeney/gator@latest

## Configuration
Gator requires a configuration file to connect to Postgres and manage its operations. Create a JSON file named `config.json` in the directory where you plan to run Gator. A sample `config.json` looks like this:

```json
{
  "database": {
    "host": "localhost",
    "port": 5432,
    "user": "postgres",
    "password": "yourpassword",
    "dbname": "gator_db"
  }
}
```
## Running Gator

Once you’ve installed the Gator CLI using `go install` and created your configuration file (`config.json`), you can start using it. Here are the commands currently implemented:

Note that all blanks denote a user input

1. **Initialize the system**:
   ```bash
   gator init

2. register _____ - Registers a new user by name (user input)

3. login _____ - Login with an existing user by name (user input)

4. users - Lists all users

5. reset - Deletes all users

6. addfeed _____ ______ - adds a new feed with name (user input 1) by URL (user input 2)

7. feeds - lists all feeds in the database

8. follow _____ - adds a feed to the current user's following list by URL (user input )

9. unfollow _____ - removes a feed from the current user's following list by URL (user input)

10. following - lists all feeds on the current user's following list

11. agg _____ - iterates through each feed on the current user's following list and creates posts based off feed data, specified buffer time between requests (user input)

12. browse (limit _______) - iterates through posts compiled via the agg command and prints out their relebant info. An optional parameter limit can control the number of posts printed (user input)