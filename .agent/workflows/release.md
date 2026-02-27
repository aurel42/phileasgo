---
description: Release workflow for PhileasGo
---

ONLY COMMIT OR RELEASE WHEN PROMPTED BY THE USER. Permission for a release can NOT be given implicitly (e.g. by simply including it in a task list or an implementation plan).
This workflow guides the release process for PhileasGo, ensuring code quality, versioning, and documentation.
0.  **Test Coverage**:
    - Ensure that we have good coverage using table-driven tests for the new or changed functionality.
1.  **Bump Version**:
    - Open [pkg/version/version.go](cci:7://file:///c:/Users/aurel/Projects/phileasgo/pkg/version/version.go:0:0-0:0).
    - Increment the patch version (e.g., `v0.2.71` -> `v0.2.72`).
    - Open [internal/ui/web/package.json](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/package.json) and bump the version there as well to match.
    - Open [msfs/efb-phileas/PackageSources/phileas/package.json](file:///c:/Users/aurel/Projects/phileasgo/msfs/efb-phileas/PackageSources/phileas/package.json) and bump the version there as well.
    - Run `npm install` in `internal/ui/web` and `msfs/efb-phileas/PackageSources/phileas` to update `package-lock.json`.
2.  **Run Tests**: Ensure all tests pass before proceeding.
    ```bash
    make test
    ```
3.  **Update History**:
    - Check git diff to ensure you're aware of all changes since the last commit.
    - Open [CHANGELOG.md](file:///c:/Users/aurel/Projects/phileasgo/CHANGELOG.md).
    - Add a new entry at the top for the new version in `vX.Y.Z (YYYY-MM-DD)` format.
    - **Patch Note Guidelines**:
        - **Focus on macroscopic changes**: Prioritize high-level features and major bug fixes.
        - Always explicitly mention when a new feature or component is added. Never bury the lead of a new feature under a list of Improvements or refactors.
        - **Omit Internal Details**: Avoid mentioning CSS properties, alignment adjustments, specific font choices, or internal code refactors. No one cares about your padding.
        - **Omit "Homework"**: Never mention that tests passed, build succeeded, or mention test coverage. Stability and testing are expected defaults, not features.
        - **No Headlines**: Do not use bold headlines within bullet points (e.g., avoid `- **Feature**: **Name**.`). Just state the change directly: `- **Feature**: Added X to Y.`.
        - **Be Concise**: Use single, punchy bullet points. Avoid "why" statements, editorializing, or justifying your design choices.
        - **Audience**: Write for a user who hasn't seen the code, not for a collaborator who sat through the dev session.
        - **Fix vs Feature vs Improvement**:
            - **Fix**: If it was broken, missing, or behaving unexpectedly, it is a **Fix**. Even if you rewrote the entire subsystem to fix it, it is still just a **Fix**.
            - **Note**: Removing hardcoded strings (e.g. backend URLs) or internal constants is ALWAYS a **Fix**, never an Improvement.
            - **Feature**: Only for entirely new capabilities that did not exist before (e.g. valid user could not do X, now they can).
            - **Improvement**: Only for existing features that work *better* (e.g. faster, prettier, easier), not for features that were broken.
        - **Symptom-Based Description**:
            - Describe the **symptom** the user experienced, not the **solution** you implemented.
            - **Bad**: "Refactored the offset logic to use geodesic distance."
            - **Good**: "Fixed formation balloons appearing in the wrong location."
            - **Omit Intermediary Steps**: Do not mention fixes for bugs or regressions that you introduced and fixed within the same development session. The user only cares about the final state relative to the *previous* version.
        - **No Selling**: 
            - Use a dry, factual, and direct tone.
            - Avoid hyperbolic or marketing language: "professional", "premium", "smart", "intelligent".
            - **Bad**: "Implemented a professional cross-fading system for a premium feel."
            - **Good**: "Added volume fades to audio actions to eliminate clicks."

STOP HERE TO LET ME REVIEW THE PATCH NOTES.

5.  **Commit**:
    - Commit all changes with a descriptive message.
    ```bash
    git add .
    git commit -m "vX.Y.Z: added feat a, fixed feat b"
    ```
    - Tag the commit with the version number in the format ""vX.Y.Z"
6.  **Done**: The release is now ready to be pushed.
DO NOT PUSH THE RELEASE. THE USER WILL DECIDE WHEN TO PUSH.
