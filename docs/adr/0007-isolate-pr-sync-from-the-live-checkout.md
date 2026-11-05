# Isolate PR sync from the live checkout

A vault checkout can hold unpublished work across several investigations. Switching it to a sync branch makes those files a condition for publishing an unrelated note. With an origin, a named sync uses a draft seeded from the published base and an alternate Git index that checkpoints explicit paths. A review identifies the candidate commit, and finish checks that candidate against the current upstream before returning its exact push refspec.

The draft is a materialized tree inside the repository's local Git directory. A second worktree would provide normal Git commands but introduce another live vault identity and checkout. The isolated draft has no Git identity of its own and contains no pending files from the live checkout unless the author explicitly imports the selected published case.
