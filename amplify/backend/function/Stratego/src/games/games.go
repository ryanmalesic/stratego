package games

import (
	"context"
	"encoding/json"
	"fmt"
	"lambda/utils"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/iotdataplane"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Board = map[int]PieceData
type Piece string
type Player string
type Result string
type Status string

type Game struct {
	PK   string `json:"pk" dynamodbav:"pk"`
	SK   string `json:"sk" dynamodbav:"sk"`
	ID   string `json:"id" dynamodbav:"id"`
	Type string `json:"type" dynamodbav:"type"`
	TTL  int64  `json:"ttl" dynamodbav:"ttl"`

	Board  Board   `json:"board" dynamodbav:"board"`
	Host   *string `json:"host" dynamodbav:"host"`
	Guest  *string `json:"guest" dynamodbav:"guest"`
	Status Status  `json:"status" dynamodbav:"status"`
}

type GamesService struct {
	dynamodbClient *dynamodb.Client
	iotClient      *iotdataplane.Client
}

type PieceData struct {
	Piece    Piece  `json:"piece" dynamodbav:"piece"`
	Player   Player `json:"player" dynamodbav:"player"`
	Revealed bool   `json:"revealed" dynamodbav:"revealed"`
}

const (
	Empty      Piece = "empty"
	Spy        Piece = "spy"
	Scout      Piece = "scout"
	Miner      Piece = "miner"
	Sergeant   Piece = "sergeant"
	Lieutenant Piece = "lieutenant"
	Captain    Piece = "captain"
	Major      Piece = "major"
	Colonel    Piece = "colonel"
	General    Piece = "general"
	Marshal    Piece = "marshal"
	Bomb       Piece = "bomb"
	Flag       Piece = "flag"
)

const (
	None  Player = "none"
	Host  Player = "host"
	Guest Player = "guest"
)

const (
	Setup     Status = "setup"
	HostMove  Status = "host"
	GuestMove Status = "guest"
	Done      Status = "done"
)

const (
	Started Result = "started"
	Moves   Result = "moves"
	Attacks Result = "attacks"
	Defends Result = "defends"
	Reveals Result = "reveals"
	Wins    Result = "wins"
)

var wins = map[Piece][]Piece{
	Spy:        {Marshal, Flag},
	Scout:      {Spy, Flag},
	Miner:      {Scout, Spy, Bomb, Flag},
	Sergeant:   {Miner, Scout, Spy, Flag},
	Lieutenant: {Sergeant, Miner, Scout, Spy, Flag},
	Captain:    {Lieutenant, Sergeant, Miner, Scout, Spy, Flag},
	Major:      {Captain, Lieutenant, Sergeant, Miner, Scout, Spy, Flag},
	Colonel:    {Major, Captain, Lieutenant, Sergeant, Miner, Scout, Spy, Flag},
	General:    {Colonel, Major, Captain, Lieutenant, Sergeant, Miner, Scout, Spy, Flag},
	Marshal:    {General, Colonel, Major, Captain, Lieutenant, Sergeant, Miner, Scout, Spy, Flag},
}

func isValidPiece(piece Piece) bool {
	switch piece {
	case Spy, Scout, Miner, Sergeant, Lieutenant, Captain, Major, Colonel, General, Marshal, Bomb, Flag:
		return true
	default:
		return false
	}
}

func NewGamesService() *GamesService {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	dynamodbClient := dynamodb.NewFromConfig(cfg)
	iotClient := iotdataplane.NewFromConfig(cfg)

	return &GamesService{dynamodbClient: dynamodbClient, iotClient: iotClient}
}

func (gs *GamesService) CreateGame(c *gin.Context) {
	c.Header("Access-Control-Allow-Methods", "*")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Header("Access-Control-Allow-Origin", "*")

	type Input struct {
		StartingPositions map[int]Piece `json:"startingPositions"`
	}

	var input Input
	err := c.BindJSON(&input)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err = validatePlayerPieces(input.StartingPositions, Host)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var playerID string
	if utils.InLambda() {
		apigwContext, ok := ginadapter.GetAPIGatewayContextFromContext(c.Request.Context())
		if !ok {
			err = fmt.Errorf("could not get apigw context")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		playerID = apigwContext.Identity.CognitoIdentityID
	} else {
		playerID = string(Host)
	}

	id := uuid.NewString()

	board := make(Board, 100)
	for i := 0; i < 40; i++ {
		board[i] = *NewPieceData(input.StartingPositions[i], Host)
	}
	for i := 40; i < 100; i++ {
		board[i] = *NewPieceData(Empty, None)
	}

	game := Game{
		PK:   fmt.Sprintf("GAME#%s", id),
		SK:   "GAME",
		ID:   id,
		Type: "GAME",
		TTL:  time.Now().AddDate(0, 0, 1).Unix(),

		Board:  board,
		Host:   aws.String(playerID),
		Guest:  nil,
		Status: Setup,
	}

	item, err := attributevalue.MarshalMap(game)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	_, err = gs.dynamodbClient.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(os.Getenv("STORAGE_STRATEGO_NAME")),
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	type Output struct {
		ID string `json:"id"`
	}

	c.JSON(http.StatusCreated, Output{ID: game.ID})
}

func (gs *GamesService) JoinGame(c *gin.Context) {
	c.Header("Access-Control-Allow-Methods", "*")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Header("Access-Control-Allow-Origin", "*")

	type Input struct {
		StartingPositions map[int]Piece `json:"startingPositions"`
	}

	var input Input
	err := c.BindJSON(&input)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err = validatePlayerPieces(input.StartingPositions, Guest)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var playerID string
	if utils.InLambda() {
		apigwContext, ok := ginadapter.GetAPIGatewayContextFromContext(c.Request.Context())
		if !ok {
			err = fmt.Errorf("could not get apigw context")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		playerID = apigwContext.Identity.CognitoIdentityID
	} else {
		playerID = string(Guest)
	}

	id := c.Params.ByName("id")

	key, err := attributevalue.MarshalMap(map[string]string{"pk": fmt.Sprintf("GAME#%s", id), "sk": "GAME"})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	item1, err := gs.dynamodbClient.GetItem(c.Request.Context(), &dynamodb.GetItemInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(os.Getenv("STORAGE_STRATEGO_NAME")),
		Key:            key,
	})
	if err != nil || item1 == nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var game Game
	err = attributevalue.UnmarshalMap(item1.Item, &game)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	board := make(Board, 100)
	for k, v := range game.Board {
		board[k] = v
	}
	for i := 40; i < 60; i++ {
		board[i] = *NewPieceData(Empty, None)
	}
	for k, v := range input.StartingPositions {
		board[k] = *NewPieceData(v, Guest)
	}

	game.Board = board
	game.Guest = aws.String(playerID)
	game.Status = HostMove

	item2, err := attributevalue.MarshalMap(game)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	_, err = gs.dynamodbClient.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("STORAGE_STRATEGO_NAME")),
		Item:      item2,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	payload, err := json.Marshal(map[string]string{"message": string(Started)})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	res, err := gs.iotClient.Publish(c.Request.Context(), &iotdataplane.PublishInput{
		ContentType: aws.String("application/json"),
		Payload:     payload,
		Topic:       aws.String(fmt.Sprintf("games/%s/moves", game.ID)),
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	fmt.Printf("Res: %v", *res)

	type Output struct {
		ID string `json:"id"`
	}

	c.JSON(http.StatusCreated, Output{ID: game.ID})
}

func (gs *GamesService) Move(c *gin.Context) {
	c.Header("Access-Control-Allow-Methods", "*")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Header("Access-Control-Allow-Origin", "*")

	type Input struct {
		From int `json:"from"`
		To   int `json:"to"`
	}

	var input Input
	err := c.BindJSON(&input)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := c.Params.ByName("id")

	key, err := attributevalue.MarshalMap(map[string]string{"pk": fmt.Sprintf("GAME#%s", id), "sk": "GAME"})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	item1, err := gs.dynamodbClient.GetItem(c.Request.Context(), &dynamodb.GetItemInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(os.Getenv("STORAGE_STRATEGO_NAME")),
		Key:            key,
	})
	if err != nil || item1 == nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var game Game
	err = attributevalue.UnmarshalMap(item1.Item, &game)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var playerID string
	if utils.InLambda() {
		apigwContext, ok := ginadapter.GetAPIGatewayContextFromContext(c.Request.Context())
		if !ok {
			err = fmt.Errorf("could not get apigw context")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		playerID = apigwContext.Identity.CognitoIdentityID
	} else {
		if game.Status == HostMove {
			playerID = string(Host)
		} else {
			playerID = string(Guest)
		}
	}

	if game.Host == nil || game.Guest == nil || game.Status == Setup {
		err = fmt.Errorf("game has not started")
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	var player Player
	if playerID == *game.Host {
		player = Host
	} else {
		player = Guest
	}

	if *game.Host != playerID && *game.Guest != playerID {
		err = fmt.Errorf("user is not a player of this game")
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	if (*game.Host == playerID && game.Status != HostMove) || (*game.Guest == playerID && game.Status != GuestMove) {
		err = fmt.Errorf("it is not the %s's turn", player)
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	result, err := Move(input.From, input.To, player, game)
	if err != nil {
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	var message string
	switch *result {
	case Moves, Attacks:
		game.Board[input.To] = PieceData{
			Piece:    game.Board[input.From].Piece,
			Player:   game.Board[input.From].Player,
			Revealed: game.Board[input.From].Revealed,
		}
		game.Board[input.From] = *NewPieceData(Empty, None)
		message = fmt.Sprintf("%s %d %d", (*result), input.From, input.To)
	case Defends:
		game.Board[input.From] = *NewPieceData(Empty, None)
		message = fmt.Sprintf("%s %d %d", (*result), input.From, input.To)
	case Reveals:
		game.Board[input.From] = *NewPieceData(Empty, None)
		game.Board[input.To] = PieceData{
			Piece:    game.Board[input.To].Piece,
			Player:   game.Board[input.To].Player,
			Revealed: true,
		}
		message = fmt.Sprintf("%s %d %d %s", (*result), input.From, input.To, game.Board[input.To].Piece)
	case Wins:
		c.JSON(http.StatusCreated, map[string]string{})
		return
	}

	if game.Status == HostMove {
		game.Status = GuestMove
	} else {
		game.Status = HostMove
	}

	item2, err := attributevalue.MarshalMap(game)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	_, err = gs.dynamodbClient.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("STORAGE_STRATEGO_NAME")),
		Item:      item2,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	payload, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	res, err := gs.iotClient.Publish(c.Request.Context(), &iotdataplane.PublishInput{
		ContentType: aws.String("application/json"),
		Payload:     payload,
		Topic:       aws.String(fmt.Sprintf("games/%s/moves", game.ID)),
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	fmt.Printf("Res: %v", *res)

	c.JSON(http.StatusCreated, map[string]string{})
}

func NewPieceData(piece Piece, player Player) *PieceData {
	return &PieceData{
		Piece:    piece,
		Player:   player,
		Revealed: false,
	}
}

func validatePlayerPieces(startingPositions map[int]Piece, player Player) error {
	var start int
	var end int
	counts := make(map[Piece]int)

	if player == Host {
		start = 0
		end = 40
	} else {
		start = 60
		end = 100
	}

	for i := start; i < end; i++ {
		piece, found := startingPositions[i]
		if !found {
			err := fmt.Errorf("piece %d is missing", i)
			return err
		}

		if !isValidPiece(piece) {
			err := fmt.Errorf("piece %s is not valid", piece)
			return err
		}

		count, found := counts[piece]
		if found {
			counts[piece] = count + 1
		} else {
			counts[piece] = 1
		}
	}

	if counts[Spy] != 1 || counts[Scout] != 8 || counts[Miner] != 5 || counts[Sergeant] != 4 || counts[Lieutenant] != 4 || counts[Captain] != 4 || counts[Major] != 3 || counts[Colonel] != 2 || counts[General] != 1 || counts[Marshal] != 1 || counts[Bomb] != 6 || counts[Flag] != 1 {
		err := fmt.Errorf("number of pieces (%v) is not valid", counts)
		return err
	}

	return nil
}

func Move(from int, to int, player Player, game Game) (*Result, error) {
	if from < 0 || from > 99 || to < 0 || to > 99 {
		err := fmt.Errorf("piece is not on the board")
		return nil, err
	}

	fromPiece, found := game.Board[from]

	if !found {
		err := fmt.Errorf("piece is not in this position")
		return nil, err
	}
	if fromPiece.Piece == Bomb || fromPiece.Piece == Flag {
		err := fmt.Errorf("%s can not move", fromPiece.Piece)
		return nil, err
	}
	if fromPiece.Player != player {
		err := fmt.Errorf("piece is not the %s's", player)
		return nil, err
	}
	if from == to {
		err := fmt.Errorf("%s must move", fromPiece.Piece)
		return nil, err
	}

	vertical := math.Abs(float64(to/10-from/10)) != 0.0
	horizontal := math.Abs(float64(to%10-from%10)) != 0.0

	if horizontal && vertical {
		err := fmt.Errorf("piece can not move diagonally")
		return nil, err
	}

	var through []int
	if vertical {
		through = utils.MakeRange10(int(math.Min(float64(to), float64(from))), int(math.Max(float64(to), float64(from))))
	} else {
		through = utils.MakeRange(int(math.Min(float64(to), float64(from))), int(math.Max(float64(to), float64(from))))
	}

	if len(through) > 2 && fromPiece.Piece != Scout {
		err := fmt.Errorf("%s can not move more than one space", fromPiece.Piece)
		return nil, err
	}

	for i, s := range through {
		if s == 42 || s == 43 || s == 46 || s == 47 || s == 52 || s == 53 || s == 56 || s == 57 {
			err := fmt.Errorf("piece can not move through water")
			return nil, err
		}
		if game.Board[s].Piece != Empty && i != 0 && i != len(through)-1 {
			err := fmt.Errorf("piece can not move through other piece")
			return nil, err
		}
	}

	var result Result
	toPiece := game.Board[to]
	switch toPiece.Player {
	case player:
		err := fmt.Errorf("piece can not end on another piece owned by the %s", player)
		return nil, err
	case None:
		result = Moves
		return &result, nil
	default:
		for _, beats := range wins[fromPiece.Piece] {
			if beats == toPiece.Piece && toPiece.Piece == Flag {
				result = Wins
			}
			if beats == toPiece.Piece {
				result = Attacks
			}
		}
		if len(result) == 0 {
			if fromPiece.Piece == Scout {
				result = Reveals
			} else {
				result = Defends
			}
		}
	}

	return &result, nil
}
