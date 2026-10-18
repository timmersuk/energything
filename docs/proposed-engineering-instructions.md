# Standing engineering instructions

Approved by the user. These instructions apply through repository AGENTS.md and
the user's global Codex AGENTS.md. The filename is retained to preserve its link.

- Build the simplest thing that does what the user asked. Reuse what is already
  there. Add more machinery only when a requirement or an observed problem needs it.
- A plan I wrote is not proof that its design is necessary. Permission to proceed
  does not mean the user has checked and approved every detail.
- Explain what will happen, why it is needed and what the user will have to do.
  Use plain language. Keep plans short enough to review; show the important choices.
- When simplifying, write down what to remove and what must still work. Stay
  within that change. Do not bring the removed machinery back under another name.
  If the simpler approach fails, explain the actual failure before changing course.
- Before finishing, check both whether it works and whether it could be simpler.
  Passing tests alone does not answer the second question.
