import { API } from "@aws-amplify/api";
import { PubSub } from "@aws-amplify/pubsub";
import { defineStore } from "pinia";
import { useRouter } from "vue-router";

const PIECES = [
  "empty",
  "spy",
  "scout",
  "miner",
  "sergeant",
  "lieutenant",
  "captain",
  "major",
  "colonel",
  "general",
  "marshal",
  "bomb",
  "flag",
] as const;

type Piece = (typeof PIECES)[number];

const PIECE_COUNTS: { [key: string]: number } = {
  spy: 1,
  scout: 8,
  miner: 5,
  sergeant: 4,
  lieutenant: 4,
  captain: 4,
  major: 3,
  colonel: 2,
  general: 1,
  marshal: 1,
  bomb: 6,
  flag: 1,
} as const;

type Player = "host" | "guest" | "none";
type BoardPlace = { piece: Piece; player: Player; revealed: boolean };
type Board = BoardPlace[];

type State = {
  gameId: string | undefined;
  selectedBoardPiece: number;
  board: Board;
  player: Player;
  turn: Player;
  status: string | undefined;
};

function setupBoard(): Board {
  return [
    ...[...Array(40).keys()].map<BoardPlace>(() => ({
      piece: "empty",
      player: "host",
      revealed: false,
    })),
    ...[...Array(20).keys()].map<BoardPlace>(() => ({
      piece: "empty",
      player: "none",
      revealed: false,
    })),
    ...[...Array(40).keys()].map<BoardPlace>(() => ({
      piece: "empty",
      player: "guest",
      revealed: false,
    })),
  ];
}

let subscription: any;

export const useBoardStore = defineStore("board", {
  state: (): State => {
    const router = useRouter();
    const gameId = router.currentRoute.value.params.id as string | undefined;
    const player = gameId ? "guest" : "host";
    const board = setupBoard();

    return {
      gameId: gameId,
      selectedBoardPiece: -1,
      board,
      player,
      turn: player,
      status: undefined,
    };
  },
  getters: {
    pieces() {
      return PIECES;
    },
  },
  actions: {
    canMove(from: number, to: number) {
      if (!this.status) {
        return false
      }

      const piece = this.board[from].piece;
      if (piece === "bomb" || piece === "flag") {
        return false;
      }

      if (from === to) {
        return false;
      }

      const vertical = Math.trunc(to / 10) - Math.trunc(from / 10) !== 0;
      const horizontal = (to % 10) - (from % 10) !== 0;

      if (vertical && horizontal) {
        return false;
      }

      let through: number[] = [];
      if (vertical) {
        through = [
          ...[...Array(Math.max(to, from) - Math.min(to, from)).keys()]
            .map((i) => i + to)
            .filter((i) => i % 10 === to % 10),
          from,
        ].sort((a, b) => a - b);
      } else {
        through = [
          ...[...Array(Math.max(to, from) - Math.min(to, from)).keys()].map(
            (i) => i + to
          ),
          from,
        ].sort((a, b) => a - b);
      }

      if (through.length > 2 && piece !== "scout") {
        return false;
      }

      for (let i = 0; i < through.length; i++) {
        if (this.isWaterSpace(through[i])) {
          return false;
        }
        if (
          this.board[through[i]].piece !== "empty" &&
          i !== 0 &&
          i !== through.length - 1
        ) {
          return false;
        }
      }

      const toPiece = this.board[to];
      if (toPiece.player === this.player) {
        return false;
      }

      return true;
    },
    canPlacePiece(piece: Piece) {
      return (
        this.board.filter((bp) => bp.piece === piece).length ===
        PIECE_COUNTS[piece]
      );
    },
    canSelectBoardPiece(index: number) {
      return (
        this.board[index].player === this.player && this.turn === this.player
      );
    },
    getPieceName(piece: Piece) {
      return piece === "empty"
        ? " "
        : piece === "spy" || piece === "bomb" || piece === "flag"
          ? piece.charAt(0).toLocaleUpperCase()
          : PIECES.findIndex((p) => p === piece);
    },
    handleClickBenchPiece(index: number) {
      if (
        this.status !== undefined ||
        this.selectedBoardPiece === -1 ||
        this.board.filter((bp) => bp.piece === PIECES[index]).length ===
        PIECE_COUNTS[PIECES[index]]
      ) {
        return;
      }

      this.board[this.selectedBoardPiece] = {
        piece: PIECES[index],
        player: this.player,
        revealed: false,
      };
    },
    async handleClickBoardPiece(index: number) {
      if (this.turn !== this.player) {
        return;
      }

      if (
        this.status === undefined &&
        this.board[index].player !== this.player
      ) {
        this.selectedBoardPiece =
          this.selectedBoardPiece === index ? -1 : index;
      } else {
        if (
          this.selectedBoardPiece === -1 &&
          this.board[index].player === this.player
        ) {
          this.selectedBoardPiece = index;
        } else {
          if (this.canMove(this.selectedBoardPiece, index)) {
            await this.movePiece(this.selectedBoardPiece, index);
            this.selectedBoardPiece = -1;
          } else {
            this.selectedBoardPiece = index;
          }
        }
      }
    },
    isEnemyPiece(index: number) {
      return this.player === "host"
        ? this.board[index].player === "guest"
        : this.player === "guest"
          ? this.board[index].player === "host"
          : false;
    },
    isWaterSpace(index: number) {
      return (
        index === 42 ||
        index === 43 ||
        index === 46 ||
        index === 47 ||
        index === 52 ||
        index === 53 ||
        index === 56 ||
        index === 57
      );
    },
    generateRandomBoard() {
      let count = 0;
      let piece = 1;
      for (let i = 0; i < 40; i++) {
        this.board[this.player === "host" ? i : i + 60] = {
          piece: PIECES[piece],
          player: this.player,
          revealed: false,
        };

        count++;

        if (PIECE_COUNTS[PIECES[piece]] === count) {
          count = 0;
          piece++;
        }
      }
    },
    async startGame() {
      if (
        this.gameId !== undefined ||
        this.player !== "host" ||
        this.turn !== "host" ||
        this.board
          .slice(0, 40)
          .map((bp) => bp.piece)
          .filter((p) => p === "empty").length > 0
      ) {
        return;
      }

      const result = await API.post("Stratego", "/games", {
        body: {
          startingPositions: this.board
            .slice(0, 40)
            .reduce<{ [key: number]: Piece }>(
              (prev, curr, index) => ({ ...prev, [index]: curr.piece }),
              {}
            ),
        },
      });

      this.gameId = result.id;

      subscription = PubSub.subscribe(`games/${this.gameId}/moves`).subscribe({
        next: (data) => {
          this.processMessage(data.value.message);
        },
        error: (error) => console.error(error),
      });
      this.turn = "guest";
      await this.$router.push(`/games/${this.gameId}`);
    },
    async joinGame() {
      if (
        this.gameId === undefined ||
        this.player !== "guest" ||
        this.turn !== "guest" ||
        this.board
          .slice(60)
          .map((bp) => bp.piece)
          .filter((p) => p === "empty").length > 0
      ) {
        return;
      }

      const result = await API.post("Stratego", `/games/${this.gameId}`, {
        body: {
          startingPositions: this.board
            .slice(60)
            .reduce<{ [key: number]: Piece }>(
              (prev, curr, index) => ({ ...prev, [index + 60]: curr.piece }),
              {}
            ),
        },
      });

      this.gameId = result.id;

      subscription = PubSub.subscribe(`games/${this.gameId}/moves`).subscribe({
        next: (data) => {
          this.processMessage(data.value.message);
        },
        error: (error) => console.error(error),
      });
      this.turn = "host";
      this.status = "started";
    },
    async movePiece(from: number, to: number) {
      await API.post("Stratego", `/games/${this.gameId}/moves`, {
        body: {
          from: from,
          to: to,
        },
      });
    },
    processMessage(message: string) {
      const split = message.split(" ");
      const result = split[0]!;

      switch (result) {
        case "started":
          this.status = "started";
          break;
        case "moves":
          this.board[parseInt(split[2])] = {
            ...this.board[parseInt(split[1])],
            player:
              this.turn === this.player
                ? this.player
                : this.player === "host"
                  ? "guest"
                  : "host",
          };
          this.board[parseInt(split[1])] = {
            piece: "empty",
            player: "none",
            revealed: false,
          };
          break;
        case "attacks":
          this.board[parseInt(split[2])] = {
            ...this.board[parseInt(split[1])],
            player:
              this.turn === this.player
                ? this.player
                : this.player === "host"
                  ? "guest"
                  : "host",
          };
          this.board[parseInt(split[1])] = {
            piece: "empty",
            player: "none",
            revealed: false,
          };
          break;
        case "defends":
          this.board[parseInt(split[1])] = {
            piece: "empty",
            player: "none",
            revealed: false,
          };
          break;
        case "reveals":
          this.board[parseInt(split[2])] = {
            ...this.board[parseInt(split[2])],
            piece: split[3] as Piece,
            revealed: true,
          };
          this.board[parseInt(split[1])] = {
            piece: "empty",
            player: "none",
            revealed: false,
          };
          break;
        case "wins":
          break;
      }
      this.turn = this.turn === "host" ? "guest" : "host";
    },
  },
});
