
# LLM command line tool for linux tasks

Claude, codex and the like are great, but sometimes you just need to remember some flags to one command real quick.  Bonus feature is finding a free model from OpenRouter for you.

>[!WARNING]
**This is primarily LLM produced code**

```bash
bash$ llm trim whitespace at the end of all lines in a file system_prompt.md
Selected free model: deepcogito/cogito-v2.1-671b
sed -i 's/[[:space:]]*$//' system_prompt.md
──────────────────────────────────────────────────────────────────────
Execute this command? y/N y
Executing...
```

```bash
bash$ llm explain tail k8s pod logs with timestamps and previous container stat
 kubectl logs -p POD_NAME --timestamps --previous                                                                                                             
  •  -p  or  --previous : Shows logs from the previous container instance                                      
  •  --timestamps : Prefixes each log line with its timestamp                                                  
  • Replace  POD_NAME  with your actual pod name                                                               
                                                                                                               
  To continuously follow (tail) these logs, add the  -f  flag:                                                 
─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
kubectl logs -p POD_NAME --timestamps --previous
─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Execute this command? y/N
```

```bash
bash$ llm --new-session --model grok --output-format json sample JWT payload
Saved file: generated_1.json
bash$ cat generated_1.json 
{
  "sub": "1234567890",
  "name": "John Doe",
  "iat": 1516239022,
  "exp": 1516242622
}
```

```bash
bash$ llm how do I rename many files?
  This loops over all  .jpg  files, renaming each by replacing  .jpg  with  .png  (using  ${f%.jpg}  parameter 
  expansion). Adjust the pattern  *.jpg  and replacement logic as needed for your files. Use  -n  flag with  mv
  ( mv -n ) for a dry-run test first.                                                                          
─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
for f in *.jpg; do mv "$f" "${f%.jpg}.png"; done
─────────────────────────────────────────────────────
```

### Command line options
- `--help`: Display help information
- `--version`: Show tool version
- `--model`: Specify LLM model to use 
- `--config`: Path to configuration file
- `--system-prompt`: Override default system prompt
- `--session-timeout`: Set session timeout in minutes
- `--new-session`: Start a new session, flushing any existing session
- `--verbose`: Enable verbose output
- `--quiet`: Suppress non-essential output
- `--save-response`: Save response to specified file

# Features
- Support multiple LLM providers, use OpenRouter to automatically find free models
- Support input from command line arguments, files, stdin, and piped input

### Configuration
- See `example.llmrc`, plain text files with key=value pairs.
- Store LLM credentials in a dot file (e.g., ~/.llmrc)
- Searches for .llmrc in current working directory, then in home directory
- Support multiple LLM configurations

### Installation
 - `make install` or copy llm to your preferred location
 - Setup configuration as above

### Response Handling Options
- Print responses to stdout
- Format responses as:
  - Plain text
  - JSON
  - Markdown
- Extract and save LLM-generated files directly to the filesystem by default (parse code blocks in markdown format)
- Parse and execute approved commands suggested by the LLM (identified in backticks or code blocks, with explicit user confirmation required for each command)

### Session
- Maintain active sessions with LLMs for up to 1 day since last interaction
- Inspect session files to see full conversation, `~/.llm_session.json`
- Allow conversation continuity within session timeout
- Option to start a new session manually (--new-session)

### Supported LLMs
- OpenRouter
- OpenAI
- Grok
- lm-proxy

