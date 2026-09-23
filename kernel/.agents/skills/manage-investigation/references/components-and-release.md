# Component progress and production preparation

Load when a development/mixed case contains independently progressing deliverables, a production handover, or post-change observation. Keep the three global states and existing purpose values; do not add a second lifecycle or require every case to reach production.

## Component progress

Identify each relevant component through its existing `S-NNN`, `DH-NNN`, repository remote and revision, or a portable configuration/migration identity. Do not create a story or handoff merely to track a non-development component. Record its current situation and decisive source in the existing evidence register; link its next action to a question, decision or acceptance criterion. Preserve handoff binding fields as materialization identity, not deployment status.

Use the team's meaningful situation labels (for example, developing, prepared, deployed, observing, or verified) with dates and evidence. They are descriptions, not new global states or proof of approval. A component without an existing handoff needs only an evidence entry identifying what it is, its supported situation and the next relevant register ID. Update that bounded observation with attributed History; preserve earlier evidence and its time/revision scope.

Derive a brief Current state/Readiness summary from those entries. A compact table may reference component and evidence IDs, but is not a second editable source of progress. Keep `investigating` while useful work or agreed observation remains; a missing dependency for one component does not block the whole objective. Waiting for the next observation interval is not itself `blocked`.

## Prepare a production handover

1. Discover applicable operational guides through `manage-operational-workflow` and the cell catalog. Use the team's strategy, not an imposed rollout, rollback, approval hierarchy or release template.
2. Present a short proposal covering the included components/revisions, necessary actions and dependencies, who performs/confirms them, relevant verification or intervention conditions, and proposed observation. Ask only for decisions or evidence that the available guides and sources cannot settle.
3. Formalize the user's validation in existing decisions/criteria with its source and role; Git identity identifies the recorder, not an approver. Keep actionable pending items in questions. An existing release run owns its detailed checklist: link it rather than copying another plan into the case.
4. Hand authorized execution to `manage-operational-workflow`. Preparing a plan grants no deployment, data mutation, external message or remediation authority. Case write authority and production execution authority remain distinct.
5. Consume verified results without treating a merge, scheduler registration or deployment acceptance as functional success. Record the actual revision or portable artifact/configuration/migration identity, environment, observation time and limitations. When observation is relevant, have the operational owner propose [observation](../../manage-operational-workflow/references/observation.md) and agree its scope with the user.
6. After a verified production deployment, check the case's claim dispositions and canonical destinations. If eligible knowledge remains, offer a bounded **Promote**/**Absorb** and mapping action; proceed when the existing authorization covers the vault and case writes. Do not equate deployment with absorption or repeat the offer for claims already verified in the vault.

Complete preparation when the proposal is user-validated or its exact unresolved decisions are explicit. Complete the investigation only against its agreed objective: specification-only cases need no rollout; a case that includes observation remains open until the agreed coverage/conditions are evaluated. A failed or inconclusive observation is not silently accepted because its period elapsed.
