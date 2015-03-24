cron-collection-stats
=====================

### Crontab
*run process every 5 minutes.
```
*/5 * * * * ./<binary> -conf <config_path> [> <log_file_path> 2>&1]
```
### Configurations

- postgresConnection
  - connection string to mp database   
- postgresConnectionStats
  - connection string to stats database 
- log
  - log subsection
    - log.level (DEBUG/INFO/WARNING/ERROR)
