docker exec -i mongo.smartcrawl sh -c 'mongodump --verbose --uri="mongodb://localhost:27017" -u $MONGO_INITDB_ROOT_USERNAME -p $MONGO_INITDB_ROOT_PASSWORD --authenticationDatabase=admin --archive --gzip --db $STORAGE_DBNAME' > ./_backup/mongodump_$(date '+%d-%m-%Y_%H-%M-%S').gz

