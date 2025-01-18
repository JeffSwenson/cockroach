# High Level Algorithm for Import

1. Write out SST to ingest and collect a key sample for each SST. 
2. Use the key samples to determine output SSTs.
3. Distribute merge tasks based on output SSTs. Prefer to merge consecutive
   SSTs on the same node so that the merge can benefit from input SST block
   caching.


# Simple Implementation

1. Distribute input shards per-node.
2. Have each node merge all input shards.
3. While producing per-node output SSTs, produce key samples for each SST.
4. Collect key samples on the coordinator to pick output splits.
5. Distribute merge tasks based on output splits.
