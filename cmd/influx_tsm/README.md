# Converting b1 and bz1 shards to tsm1

## Steps

* Identify databases to be converted.
* Choose parallelism.
* Stop write traffic.
* Restart node and ensure all WAL data is flushed.
* Run conversion tool.
* Restart node and ensure data looks correct.
* If everything looks OK, remove or archive backups.
