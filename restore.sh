#it's possible to restore to different database, see link
#https://stackoverflow.com/questions/36321899/mongorestore-to-a-different-database/36322080#36322080
docker exec -i mongo.smartcrawl sh -c 'mongorestore --verbose --uri="mongodb://root:admin@mongo:27017/?authSource=admin&ssl=false" -u $MONGO_INITDB_ROOT_USERNAME -p $MONGO_INITDB_ROOT_PASSWORD --authenticationDatabase=admin --archive --gzip --drop' < "$@"

