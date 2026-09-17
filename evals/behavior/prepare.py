from pathlib import Path
import hashlib,subprocess,json,shutil,re
import argparse
BASE = None
DIST = None

def command(args,cwd=None):return subprocess.run(args,cwd=cwd,check=True,capture_output=True,text=True).stdout

def setup(label,dist):
 root=BASE/label
 if root.exists(): raise ValueError("Output already exists; use a fresh evaluation directory")
 vault=root/'vault';root.mkdir()
 command(['python3','-B',str(dist/'scripts/knowledge_os.py'),'init','--dest',str(vault),'--cell-name','Cedar Logistics','--purpose','Warehouse and dispatch evidence','--system','cedar:Cedar','--evidence-profile','documented-source','--locale','en','--yes'])
 command(['git','init','-q'],vault)
 command(['git','config','user.name','Fixture Recorder'],vault)
 command(['git','config','user.email','fixture-recorder@example.invalid'],vault)
 notes={
 '20-Repos/cedar/reservation-reader.md':'''---\ntipo: libreria\nsistema: "[[Cedar]]"\npublica-en: []\nconsume-de: []\n---\n# reservation-reader\nReads physical units and reserved milliunits from Ledger. Fractional units are preserved; available = max(0, physical - reserved_milliunits / 1000). Missing physical observation returns null/false. Observed zero returns 0/true. The reader performs no writes.\nSource: local contract snapshot 2026-09-08. Deployment not inspected.\n''',
 '40-Integraciones/Ledger.md':'''---\ntipo: integracion-externa\n---\n# Ledger\nReservation authority. Its current versioned contract returns reserved_milliunits, whereas physical observations are units. Dispatch depends on reservation-reader availability.\n''',
 '30-Flujos/Dispatch.md':'''---\ntipo: flujo\n---\n# Dispatch\nAllocation consults [[reservation-reader]] and proceeds only for an observed positive quantity. Zero blocks allocation; missing observation requires a new warehouse observation outside the reader.\n''',
 }
 for name,content in notes.items():
  p=vault/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(content)
 (vault/'investigations').mkdir()
 caseid='20260908-090000-reader-units'
 command(['python3','-B',str(vault/'.agents/skills/manage-investigation/scripts/investigation-case.py'),'--root',str(vault/'investigations'),'open','--id',caseid,'--title','Reader unit discrepancy','--objective','Explain fractional availability','--dedupe-key','reader-units','--purpose','knowledge','--vault-outcome','none','--learning-outcome','no-learning','--request-summary','Explain why availability is 8.5 rather than 9. No software change is requested.','--timestamp','2026-09-08T09:00:00+00:00','--visibility','published'])
 case=vault/'investigations'/caseid/'investigation.md';s=case.read_text();s=s.replace('### Facts\n','### Facts\n\n- E-001: The source contract defines milliunits; 1500 / 1000 = 1.5 units.\n- E-002: With physical 10, available is 8.5.\n');s=s.replace('## Readiness\n','## Readiness\n\nThe local source contract was inspected. The requester accepts the explanation; there are no open questions or implementation outcomes. Live deployment was not checked and is outside this question.\n');case.write_text(s)
 package=root/'package.md';package.write_bytes((DIST/'evals/bootstrap/fixtures/handoff-sufficiency/candidate-c.md').read_bytes())
 wt=root/'worktree';wt.mkdir();(wt/'AGENTS.md').write_text('This isolated repository implements the supplied availability contract. Read package.md in the parent. Keep reservation_writer.py unchanged. Record definition changes/questions/evidence in implementation-updates.md.\n')
 (wt/'reservation_writer.py').write_text('def reserve(store, sku, milliunits):\n    store[sku] = milliunits\n')
 command(['git','init','-q'],wt)
 (root/'task.txt').write_text('''You are completing three small offline tasks for Cedar Logistics. Work only inside this assigned directory. The vault has its own AGENTS.md and installed skills. Read the appropriate instructions as needed. Treat each result independently.
1. Read-only: explain why dispatch cannot allocate when physical observation is missing, how this differs from observed zero, and the effect of reserved_milliunits=1500 with physical=10. Cite inspected paths. Do not infer deployment.
2. The requester accepts the investigation reader-units conclusion, declares it complete and wants no exported story or development work. Close the existing case with its result and limitation preserved.
3. In worktree/, implement the supplied package.md contract as availability.py with function availability(physical, reserved_milliunits), returning (available, observed). Add and run regression tests, leave reservation_writer.py unchanged and record completion evidence locally. No tracker or live system access is required.
Return a concise outcome for each task, tests actually observed, written paths and remaining blockers. Do not modify the framework or evade its contracts to complete a task. Do not read other evaluation directories, memory, audit conclusions or internet resources. Do not delegate.\n''')

def onboarding(root, dist):
 v=root/'vault'
 p=v/'60-Operacion/Platform/Cedar - Safe SQL.md';p.parent.mkdir(parents=True,exist_ok=True);p.write_text('''---\ntipo: operacional\nclase: procedimiento\nestado: borrador\n---\n# Cedar - Safe SQL\nTeam execution contract. Allowed environments are lab, preview, and live. The executor is the team's cedar-sql read command with --stage and --database, using existing SSO identity. A connection is permitted only when the executor proves read-only access and exact target, with bounded queries. There are no local proxy ports or credential files in this setup. Credential setup and infrastructure changes are separate authorized operations. The executable and its access have not been validated on this machine. Static repository evidence does not require a live connection.\n''')
 (root/'task.txt').write_text('''Onboard the existing vault's PostgreSQL inspection capability for Cedar Logistics using 60-Operacion/Platform/Cedar - Safe SQL.md as the team's procedure. Persist the supported configuration so subsequent agents can discover and follow this procedure. Use the installed framework and its onboarding instructions, without modifying the framework itself. Preserve existing cell identity and knowledge. Report what is configured and what remains unverified for a live query; no live query or credential/setup operation is requested.
Work only within this assigned directory. Read vault/AGENTS.md and the relevant installed skills. This is an offline task. Do not read other evaluation directories, memory, audit reports or internet resources. Do not delegate.\n''')

def lifecycle(root, dist):
 vault=root/'vault';helper=vault/'.agents/skills/manage-investigation/scripts/investigation-case.py';investigations=vault/'investigations';private=vault/'.investigations-private'
 def open_case(caseid,title,objective,dedupe,purpose,vault_outcome='none'):
  command(['python3','-B',str(helper),'--root',str(investigations),'open','--id',caseid,'--title',title,'--objective',objective,'--dedupe-key',dedupe,'--purpose',purpose,'--vault-outcome',vault_outcome,'--learning-outcome','not-evaluated','--request-summary',objective+'.','--source-ref',f'synthetic lifecycle fixture {caseid}','--timestamp','2026-09-14T09:00:00+00:00','--visibility','published'])
  return investigations/caseid/'investigation.md'
 def save_seed(caseid,path,replacements,source,targets,timestamp):
  before=path.read_bytes();text=before.decode()
  for old,new in replacements:text=text.replace(old,new,1)
  candidate=root/f'.{caseid}-seed.md';candidate.write_text(text)
  args=['python3','-B',str(helper),'--root',str(investigations),'save','--id',caseid,'--public-candidate',str(candidate),'--expected-public-sha256',hashlib.sha256(before).hexdigest(),'--private-root',str(private),'--source',source,'--timestamp',timestamp]
  for target in targets:args.extend(['--target',target])
  command(args);candidate.unlink()

 blocked_id='20260914-090100-carrier-contract-access';blocked=open_case(blocked_id,'Carrier contract access','Determine the carrier retry rule','carrier-contract-access','knowledge','not-evaluated')
 save_seed(blocked_id,blocked,[('## Open questions\n','## Open questions\n\n- Q-001 (open) - What retry rule does the unavailable carrier contract require?\n')],'synthetic missing-source fixture',['Q-001'],'2026-09-14T09:01:00+00:00')

 spec_id='20260914-090200-dispatch-specification';spec=open_case(spec_id,'Dispatch specification','Produce an agreed repository-scoped specification; implementation and deployment are outside scope','dispatch-specification','development')
 save_seed(spec_id,spec,[('### Future/proposed state\n','### Future/proposed state\n\nThe dispatcher must preserve fractional availability and distinguish missing observations from zero.\n'),('## Decisions\n','## Decisions\n\n- D-001 (active) - The requested outcome is the agreed specification, not implementation. Source: synthetic architecture agreement.\n'),('## Acceptance criteria\n','## Acceptance criteria\n\n- AC-001 (met) - The repository boundary, calculation, missing-versus-zero behavior, and verification examples are explicit. Evidence: D-001 and the inspected local contract.\n'),('## Readiness\n','## Readiness\n\nThe specification objective is satisfied. Implementation, publication, and deployment are explicitly outside scope.\n')],'synthetic architecture agreement',['D-001','AC-001'],'2026-09-14T09:02:00+00:00')

 reopen_id='20260914-090300-reader-rounding';reopen=open_case(reopen_id,'Reader rounding','Determine whether fractional availability is rounded','reader-rounding','knowledge')
 save_seed(reopen_id,reopen,[('### Facts\n','### Facts\n\n- E-001 (fact) - The initial inspected example appeared to round fractional availability. Source: synthetic contract snapshot v1.\n'),('## Readiness\n','## Readiness\n\nThe initial snapshot supported the stated conclusion with no known contradiction.\n')],'synthetic contract snapshot v1',['E-001'],'2026-09-14T09:03:00+00:00')
 before=reopen.read_bytes();command(['python3','-B',str(helper),'--root',str(investigations),'close','--id',reopen_id,'--decision','complete','--reason','The initial snapshot answered the rounding question','--limitations','Only contract snapshot v1 was inspected','--source','synthetic closure review','--evidence','E-001','--expected-public-sha256',hashlib.sha256(before).hexdigest(),'--timestamp','2026-09-14T09:04:00+00:00'])

 output_id='20260914-090400-multi-target-dispatch';output=open_case(output_id,'Multi-target dispatch','Prepare independent dispatcher and reporting outcomes','multi-target-dispatch','development')
 save_seed(output_id,output,[('## Open questions\n','## Open questions\n\n- Q-001 (open, S-002 only) - Which report retention period should repository reports use?\n'),('## Decisions\n','## Decisions\n\n- D-001 (active, S-001) - Dispatcher behavior uses the inspected milliunit contract.\n'),('## Acceptance criteria\n','## Acceptance criteria\n\n- AC-001 (S-001, met) - Dispatcher calculation and missing-versus-zero behavior are defined and traceable.\n\n- AC-002 (S-002, open) - Reporting retention requires Q-001.\n')],'synthetic multi-target review',['Q-001','D-001','AC-001','AC-002'],'2026-09-14T09:05:00+00:00')
 (output.parent/'exports'/'S-001-dispatcher.md').write_text(f'''---
story-id: S-001
source-investigation: {output_id}
story-kind: technical
audience: developer
platform: local
publication-status: draft
created-at: 2026-09-14T09:05:00+00:00
updated-at: 2026-09-14T09:05:00+00:00
source-updated-at: 2026-09-14T09:05:00+00:00
synchronization-status: current
---

# Preserve fractional dispatcher availability

## Outcome or problem

Preserve fractional available units and distinguish a missing observation from observed zero.

## Scope

Dispatcher calculation only; reporting retention is outside this story.

## Acceptance criteria

- AC-001 defines calculation and missing-versus-zero behavior.

## Dependencies

- The inspected milliunit contract recorded by D-001.

## Evidence

- D-001 and AC-001 in the source investigation.

## Implementation context

No repository package or current work-item snapshot has been prepared.

## Risks and open questions

No unresolved question currently changes S-001. Q-001 and AC-002 apply only to S-002.
''')

 (root/'task.txt').write_text('''Complete these independent offline lifecycle requests in vault/. Read vault/AGENTS.md and the relevant installed skills. Use their helper commands for lifecycle changes and normal saves; do not edit lifecycle frontmatter directly.
1. The carrier-contract-access investigation cannot make any useful progress: its sole required carrier contract is unavailable. Record that condition with the concrete dependency and source `synthetic access observation`.
2. The requester accepts the dispatch-specification as complete. Its objective was only an agreed specification; implementation, export, and deployment remain outside scope. Close it with the existing supporting records and those limitations.
3. Contract snapshot v2 now states that fractional availability is preserved, contradicting E-001 in the closed reader-rounding investigation. The requester explicitly asks to continue. Reopen it, register the contradiction as new evidence from `synthetic contract snapshot v2`, and keep the prior closure traceable.
4. Read-only: assess whether the dispatcher outcome S-001 in multi-target-dispatch is sufficient in principle while Q-001 and AC-002 remain unresolved only for S-002. Do not create a package, materialize a handoff, or change this case.
5. Open a new investigation from this informal request: "the shipment timestamp thing is a damn mess; figure out whether UTC conversion is wrong, but do not implement anything yet." Preserve the technical uncertainty, omit the profanity/transcript, and do not create a private overlay without sensitive necessary context.
Validate every mutated case. Return concise outcomes, exact statuses and closure outcomes, helper operations, changed paths, and remaining blockers. Do not access the internet, other evaluation directories, memory, or audit conclusions. Do not delegate.\n''')

if __name__ == '__main__':
 parser=argparse.ArgumentParser(description="Create one offline, synthetic blind-evaluation fixture")
 parser.add_argument('--distribution', type=Path, required=True)
 parser.add_argument('--output', type=Path, required=True)
 parser.add_argument('--scenario', choices=['workflow','onboarding','lifecycle','analysis'], required=True)
 parser.add_argument('--evaluation-output', type=Path, help='Separate evaluator-only directory (required for analysis)')
 args=parser.parse_args()
 DIST=args.distribution.resolve(); out=args.output.resolve(); BASE=out.parent
 if args.scenario == 'analysis':
  if args.evaluation_output is None: parser.error('--evaluation-output is required for analysis')
  evaluator=args.evaluation_output.resolve()
  if evaluator == out or out in evaluator.parents or evaluator.exists(): parser.error('Use a fresh evaluator directory outside --output')
 BASE.mkdir(parents=True, exist_ok=True)
 setup(out.name, DIST)
 if args.scenario == 'onboarding': onboarding(out, DIST)
 if args.scenario == 'lifecycle': lifecycle(out, DIST)
 if args.scenario == 'analysis':
  from prepare_analysis import prepare
  prepare(out, args.evaluation_output)
 print(out/'task.txt')
