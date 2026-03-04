# Hot-Reloading and Testing System Prompts

Now that all LLM System Prompts have been externalized from the compiled Go binary, you have the freedom to edit, experiment, and optimize agent behavior locally without running any compilers! 

## Directory Structure
All editable AI prompts are stored in the root `src/prompts/` directory:
- `analyst_sql.md`: Text-to-SQL logic block
- `analyst_summary.md`: The SQL summarization/interpretation block
- `planner.md`: The rule engine for picking actions to take
- `strategist.md`: The Chain-of-Thought reasoning generator
- `liaison_email.md`: Professional email generation
- `liaison_slack.md`: Slack/Teams message generation
- `liaison_report.md`: Executive report generation

## How to Edit and Reload Prompts Live

The backend server reads prompts out of memory for performance. When you edit a  file on your local disk at `src/prompts/*.md`, you must tell the backend to physically dump its cache and re-read the disk.

**Step 1:** Modify the Markdown file directly in your IDE (e.g. VSCode / Cursor).
**Step 2:** Open a terminal or browser and send an HTTP GET request to the local backend's reload administration endpoint:

> [!TIP]
> **Docker Reloading:** If you are running the application via `docker-compose`, the `src/prompts` host directory is explicitly bind-mounted as a Volume into the backend container at `/app/prompts`. This means when you save changes to the markdown file on your Windows host, it instantly overrides the file within the isolated Linux backend container! Just fire the `/reload` API call as normal.

```bash
curl http://localhost:8080/api/prompts/reload
```

*Response Expected:*
```json
{"message":"System prompts successfully reloaded from disk","status":"success"}
```

**Step 3:** Perform an action in the UI that triggers that AI path. The agent will instantly inherit all the new properties, rules, constraints, and instructions you've just authored!

---

## Best Practices for Tweaking Prompts

1. **Test Incrementally**: Do not completely rewrite a prompt in one go. Instead, modify one specific paragraph or constraint, hit `/api/prompts/reload`, and submit a user message in the chat UI. Observe if the LLM followed the constraint perfectly. If not, increase the "firmness" or strictness of the wording in that block.
2. **Be Careful with Modifiers (`%s`)**: Look inside `analyst_sql.md`. You will notice a `%s` hanging in the `DATABASE SCHEMA:` section. Do **not** remove this. The Go application uses standard `fmt.Sprintf` syntax to inject database tables natively into that slot before calling the LLM. You are free to change text *around* the `%s`, just don't delete it.
3. **Reasoning Formats Matter**: Prompts like `strategist.md` and `planner.md` dictate the shape of UI cards on the frontend. Be careful changing core structures (like `ACTION:` and `Title:` in the Planner) as the backend regex or frontend components may fail to parse your modifications. Concentrate on editing the *tone* and *decision-making intent*, rather than the rigid `Code-Block` API format logic.
4. **Use Output Demonstrations (Few Shot)**: If an agent starts performing poorly, the best way to improve it is by adding "examples" of what you want inside the Markdown file. LLMs operate far better with 1 good example than 10 paragraphs of complex theoretical directions.
