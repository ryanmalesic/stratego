<script setup lang="ts">
import { useBoardStore } from "@/stores/board";
import { storeToRefs } from "pinia";

const store = useBoardStore();
const { board, pieces, selectedBoardPiece } = storeToRefs(store);
const {
  canMove,
  canPlacePiece,
  canSelectBoardPiece,
  getPieceName,
  handleClickBenchPiece,
  handleClickBoardPiece,
  isEnemyPiece,
  isWaterSpace,
  startGame,
  generateRandomBoard,
  movePiece
} = store;
</script>

<template>
  <main class="flex flex-col justify-center items-center max-w-screen-xl mx-auto min-h-screen max-h-screen p-4 gap-4">
    <div class="grid grid-cols-10 grid-rows-10 gap-0 flex-1 h-full sm:p-4 aspect-square">
      <div v-for="(pb, i) in board" :key="i" v-on:click="handleClickBoardPiece(i)"
        class="flex justify-center border border-slate-500 items-center text-2xl aspect-square" :class="{
          [`bg-green-100`]: selectedBoardPiece === -1 ? false : canMove(selectedBoardPiece, i),
          [`bg-blue-100`]: isWaterSpace(i),
          [`bg-red-100`]: isEnemyPiece(i),
          [`bg-slate-200`]:
            (!canSelectBoardPiece(i) && !isEnemyPiece(i) && !isWaterSpace(i) && !(selectedBoardPiece === -1 ? false : canMove(selectedBoardPiece, i))) ||
            selectedBoardPiece === i,
        }">
        <span>{{ getPieceName(pb.piece) }}</span>
      </div>
    </div>
    <div class="grid grid-cols-13 grid-rows-1 gap-0 h-full sm:p-4 w-full">
      <div v-for="(piece, i) in pieces" v-on:click="handleClickBenchPiece(i)" :key="piece"
        class="flex justify-center border border-slate-500 items-center text-2xl aspect-square" :class="{
          [`bg-slate-100`]: canPlacePiece(piece),
        }">
        <span>{{ getPieceName(piece) }}</span>
      </div>
    </div>
    <p><button @click="generateRandomBoard">Random</button></p>
    <p><button @click="startGame">StartGame</button></p>
  </main>
</template>
