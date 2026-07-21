# iterative_per — Plan / Execute / Review Loop

**Purpose**  
Orchestrates a bounded iterative workflow: Plan → Execute → Review. Every phase uses a fresh, isolated delegate context. Implemented in the TUI as async `tea.Cmd` stages (`internal/tui/iterative_per.go`) so Bubble Tea `Update` never blocks.

**Required Behaviour**

1. **Definition of Done**  
   Before execution begins, the agent must obtain (via planning conversation + confirm) a precise definition of "done" and write `*_definition_of_done.md` scaffolds on confirm.

2. **Supervised Mode (runtime choice)**  
   Immediately after the project name, ask:  
   `Run in supervised mode? (y/N)` — default is **N**.

   - If **N** (default): After plan confirmation, run multi-cycle Plan→Execute→Review fully autonomously in one async worker.
   - If **Y**: After plan confirmation:
     1. Run the **Plan** phase async.
     2. Output the plan using this exact tagged format:
        ```
        <!--SHIM_BRIEF_START-->
        <full brief or plan text>
        <!--SHIM_BRIEF_END-->

        Brief ready. Reply with executor: auto / coder / deep-thinker / orchestrator
        ```
     3. Stop and wait for the user to reply with an executor name (`awaiting_executor`).
     4. Only then run multi-cycle **Execute→Review** (no further re-Plan) with the chosen executor, up to `max_iterations` Execute attempts.

3. **Human Log**  
   After every phase the controller appends a single line to `per_log.txt` in the target directory:
   ```
   2026-07-03T14:22:05Z | Plan phase completed for ticket-774f
   ```

4. **Review status + multi-cycle**  
   Review final messages should include exactly one of:
   - `PER_STATUS: DONE`
   - `PER_STATUS: RETRY`
   - `PER_STATUS: FAILED`

   **Inner cycle:** up to `RetryBudget+1` Execute→Review pairs without re-Plan (default RetryBudget = **1** → two attempts).  
   **Outer multi-cycle (unsupervised full mode):** when a cycle exhausts on RETRY, **re-Plan** and continue.  
   **Hard cap:** total Execute phases ≤ `config.iterative.max_iterations` (default **40**). Then write `per_done.signal`.  
   FAILED or DONE stop immediately. Cancel / first Ctrl+C cancels the in-flight phase context and invalidates `iterGen`.

5. **Context Reset**  
   Every `delegate_task` / `DelegateTaskWithConfig` uses a unique temporary database.

**Artifacts (target dir)**  
- `per_plan.md` — latest Plan phase output  
- `per_log.txt` — human phase log  
- `per_done.signal` — terminal marker  

**Notes**  
- Executor selection currently resolves to the single active default model (`EnsureMVPExecutors`).  
- Supervised execute path does not re-Plan; it uses the plan file / PlanText from the gate.  
- Cancel / first Ctrl+C cancels the in-flight phase context and invalidates `iterGen`.
