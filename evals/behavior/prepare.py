from pathlib import Path
import subprocess,json,shutil,re
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
 notes={
 '20-Repos/cedar/reservation-reader.md':'''---\ntipo: libreria\nsistema: "[[Cedar]]"\npublica-en: []\nconsume-de: []\n---\n# reservation-reader\nReads physical units and reserved milliunits from Ledger. Fractional units are preserved; available = max(0, physical - reserved_milliunits / 1000). Missing physical observation returns null/false. Observed zero returns 0/true. The reader performs no writes.\nSource: local contract snapshot 2026-09-08. Deployment not inspected.\n''',
 '40-Integraciones/Ledger.md':'''---\ntipo: integracion-externa\n---\n# Ledger\nReservation authority. Its current versioned contract returns reserved_milliunits, whereas physical observations are units. Dispatch depends on reservation-reader availability.\n''',
 '30-Flujos/Dispatch.md':'''---\ntipo: flujo\n---\n# Dispatch\nAllocation consults [[reservation-reader]] and proceeds only for an observed positive quantity. Zero blocks allocation; missing observation requires a new warehouse observation outside the reader.\n''',
 }
 for name,content in notes.items():
  p=vault/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(content)
 (vault/'investigations').mkdir()
 caseid='20260908-090000-reader-units'
 command(['python3','-B',str(vault/'.agents/skills/manage-investigation/scripts/investigation-case.py'),'--root',str(vault/'investigations'),'open','--id',caseid,'--title','Reader unit discrepancy','--objective','Explain fractional availability','--dedupe-key','reader-units','--purpose','knowledge','--vault-outcome','none','--learning-outcome','no-learning','--request-summary','Explain why availability is 8.5 rather than 9. No software change is requested.','--timestamp','2026-09-08T09:00:00+00:00'])
 case=vault/'investigations'/caseid/'investigation.md';s=case.read_text().replace('status: intake','status: validating');s=s.replace('### Facts\n','### Facts\n\n- F-001: The source contract defines milliunits; 1500 / 1000 = 1.5 units.\n- F-002: With physical 10, available is 8.5.\n');s=s.replace('## Readiness\n','## Readiness\n\nThe local source contract was inspected. The requester accepts the explanation; there are no open questions or implementation outcomes. Live deployment was not checked and is outside this question.\n');case.write_text(s)
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
 skill=v/'.agents/skills/inspect-database'
 shutil.copytree(dist/'adapters/postgres/inspect-database',skill,dirs_exist_ok=True)
 inst=v/'instance.yaml';inst.write_text(inst.read_text().replace('adapters: []','adapters: [postgres]'))
 p=v/'60-Operacion/Platform/Cedar - Safe SQL.md';p.parent.mkdir(parents=True,exist_ok=True);p.write_text('''---\ntipo: operacional\nclase: procedimiento\nestado: borrador\n---\n# Cedar - Safe SQL\nTeam execution contract. Allowed environments are lab, preview, and live. The executor is the team's cedar-sql read command with --stage and --database, using existing SSO identity. A connection is permitted only when the executor proves read-only access and exact target, with bounded queries. There are no local proxy ports or credential files in this setup. Credential setup and infrastructure changes are separate authorized operations. The executable and its access have not been validated on this machine. Static repository evidence does not require a live connection.\n''')
 (root/'task.txt').write_text('''Onboard the existing vault's PostgreSQL inspection capability for Cedar Logistics using 60-Operacion/Platform/Cedar - Safe SQL.md as the team's procedure. Persist the supported configuration so subsequent agents can discover and follow this procedure. Use the installed framework and its onboarding instructions, without modifying the framework itself. Preserve existing cell identity and knowledge. Report what is configured and what remains unverified for a live query; no live query or credential/setup operation is requested.
Work only within this assigned directory. Read vault/AGENTS.md and the relevant installed skills. This is an offline task. Do not read other evaluation directories, memory, audit reports or internet resources. Do not delegate.\n''')

if __name__ == '__main__':
 parser=argparse.ArgumentParser(description="Create one offline, synthetic blind-evaluation fixture")
 parser.add_argument('--distribution', type=Path, required=True)
 parser.add_argument('--output', type=Path, required=True)
 parser.add_argument('--scenario', choices=['workflow','onboarding'], required=True)
 args=parser.parse_args()
 DIST=args.distribution.resolve(); out=args.output.resolve(); BASE=out.parent
 BASE.mkdir(parents=True, exist_ok=True)
 setup(out.name, DIST)
 if args.scenario == 'onboarding': onboarding(out, DIST)
 print(out/'task.txt')
