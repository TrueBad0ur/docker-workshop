docker run -d -p 5432:5432 --name postgres -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=yourpassword -e POSTGRES_DB=shishaDB postgres:16.3-bookworm

docker-compose up

cd server 
go run main.go

cd client 
npm start