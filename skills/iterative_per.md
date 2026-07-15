# iterative_per — Plan / Execute / Review Loop

**Purpose**  
Orchestrates a bounded iterative workflow: Plan → Execute → Review, returning to Plan after each Review until the goal is complete or max iterations reached. Every phase transition uses a fresh, isolated context.

**Required Behaviour**

1. **Definition of Done**  
   Before any execution begins, the agent **must** ask the user (or be given) the precise definition of "done" for the supplied goal and present a high-level plan for confirmation.

2. **Supervised Mode (runtime choice)**  
   Immediately after receiving the goal, ask the user:  
   `Run in supervised mode? (y/N)` — default is **N**.

   - If **N** (default): Run the loop fully autonomously after the initial plan approval.
   - If **Y**: After every Plan phase (and after any Review that proposes material changes), do the following:
     1. Produce the plan/brief.
     2. Output it using this exact tagged format:
        ```
        <!--SHIM_BRIEF_START-->
        <full brief or plan text>
        <!--SHIM_BRIEF_END-->

        Brief ready. Reply with executor: auto / coder / deep-thinker / orchestrator
        ```
     3. Stop and wait for the user to reply with an executor name.
     4. Only then call `delegate_task` with the chosen executor for the Execute phase.

3. **Human Log**  
   After every phase the controller appends a single line to `per_log.txt`:
   ```
   2026-07-03T14:22:05Z | Plan phase completed for ticket-774f
   ```

4. **Max Iterations**  
   Accepts `max_iterations` (default 12). On limit, write `per_done.signal`.

5. **Context Reset**  
   Every `delegate_task` uses a unique temporary database.

**MVP Notes**
- Executor selection currently resolves to the single active default model.
- When supervised mode is active, the agent must use the tagged brief format above so the user can reply with the executor name in the next turn.