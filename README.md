# item-archive-d

A lightweight web application for managing item archives with image support, built with Go and SQLite.

## Usage

Run the server using:

```sh
Usage of item-archive-d:
  -addr string
        The address to listen on. (default ":4502")
  -data string
        The directory in which to store item-archive data. (default ".")
  -migration string
        Specify a file containing migration statements to run upon opening the database. (optional)
```

**Example:**

```sh
go run . -addr :8080 -data ./my-archive
```

The application creates the following in the data directory:

- `blobs/`: Directory for storing image data.
- `state.db`: SQLite database file.

## AI Tagger

The project also includes a tool to automatically tag images using GenAI models (specifically Gemma 3 27B IT).

Run the tagger using:

```sh
go run ./internal/ai-tagger [-data]
```

The environment variable `GOOGLE_API_KEY` is required, it should be a Google Gemini API key.

The `-data` flag means the same thing as `-data` for the main server binary.

The tagger will scan for items with images but no title (specifically "Untitled*") and use the image content to generate a descriptive title using the Gemma 3 model.

## Development

Documentation for hacking on this repository can be found [here](./DEVELOPMENT.md).
