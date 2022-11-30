package gol

import (
	"fmt"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

func saveWorldAsImage(c distributorChannels, name string, height, width, turns int, world [][]uint8) {
	fmt.Println("Saving...")
	c.ioCommand <- ioOutput
	c.ioFilename <- name + "x" + strconv.Itoa(turns)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			c.ioOutput <- world[row][col]
		}
	}
}

func quitExecution(c distributorChannels, turns int) {
	fmt.Println("Quitting...")
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turns, Quitting}
	close(c.events)
}

// makeMatrix allocates memory for a matrix
func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func calcAliveCellCount(height, width int, world [][]byte) int {
	var count int
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if world[row][col] == 255 {
				count++
			}
		}
	}
	return count
}

func calcAliveCells(world [][]byte, height, width int) []util.Cell {
	var cells []util.Cell
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if world[row][col] == 255 {
				c := util.Cell{X: col, Y: row}
				cells = append(cells, c)
			}
		}
	}
	return cells
}

// pauseLoop infinite loop waiting on another 'p' key press
func pauseLoop(pause chan bool, c distributorChannels, name string, height, width, turns int, world [][]byte) {
	for {
		k := <-c.keyPresses
		if k == 'p' {
			pause <- true
			break
		} else if k == 's' {
			saveWorldAsImage(c, name, height, width, turns, world)
		} else if k == 'q' {
			quitExecution(c, turns)
			break
		}
	}
}

func worker(startY, endY int, worldIn [][]byte, out chan<- [][]uint8, p Params) {
	boardSeg := updateBoard(startY, endY, worldIn, p)
	out <- boardSeg
}

// UpdateBoard updates and returns a single iteration of GOL
func updateBoard(startY, endY int, worldIn [][]byte, p Params) [][]byte {
	segHeight := endY - startY

	// initialise worldOut with dead cells
	worldOut := makeMatrix(segHeight, p.ImageWidth)
	for row := 0; row < segHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			worldOut[row][col] = 0
		}
	}

	for row := startY; row < endY; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			// CURRENT ELEMENT AND ITS NEIGHBOR COUNT RESET
			element := worldIn[row][col]
			counter := 0

			// iterate through all neighbors of given element
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nRow := (row + dx + p.ImageHeight) % p.ImageHeight
					nCol := (col + dy + p.ImageWidth) % p.ImageWidth
					// increment counter if given neighbor is alive
					if worldIn[nRow][nCol] == 255 {
						counter++
					}
				}
			}

			// if element is alive exclude it from the counter
			if element == 255 {
				counter--
			}

			superRow := row - startY

			// if element is dead
			if element == 0 {
				if counter == 3 {
					worldOut[superRow][col] = 255
				} else {
					worldOut[superRow][col] = 0
				}
			} else {
				// if element is alive
				if counter < 2 {
					worldOut[superRow][col] = 0
				} else if counter > 3 {
					worldOut[superRow][col] = 0
				} else {
					worldOut[superRow][col] = 255
				}
			}
		}
	}
	return worldOut
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// 	INPUT operations
	name := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- name

	worldIn := makeMatrix(p.ImageHeight, p.ImageWidth)
	// get image byte by byte and store in: worldIn
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			worldIn[row][col] = <-c.ioInput
			if worldIn[row][col] != 0 {
				c.events <- CellFlipped{Cell: util.Cell{X: col, Y: row}}
			}
		}
	}

	// initialise worldOut as worldIn for turn 0
	worldOut := makeMatrix(p.ImageHeight, p.ImageWidth)
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			worldOut[row][col] = worldIn[row][col]
		}
	}

	timeOver := time.NewTicker(2 * time.Second)
	pause := make(chan bool, 1)
	quit := false
	var key rune
	turn := 0

	// TODO: Execute all turns of the Game of Life.
	for turn < p.Turns {
		select {
		case <-timeOver.C:
			if !quit {
				c.events <- AliveCellsCount{turn, calcAliveCellCount(p.ImageHeight, p.ImageWidth, worldOut)}
			}
		case key = <-c.keyPresses:
			switch key {
			case 'p':
				fmt.Println("Paused. Current turn:", turn)
				go pauseLoop(pause, c, name, p.ImageHeight, p.ImageWidth, turn, worldOut)
				c.events <- StateChange{turn, Paused}
				_ = <-pause
				c.events <- StateChange{turn, Executing}
				fmt.Println("Continuing.")
			case 's':
				saveWorldAsImage(c, name, p.ImageHeight, p.ImageWidth, p.Turns, worldOut)
			case 'q':
				quit = true
				saveWorldAsImage(c, name, p.ImageHeight, p.ImageWidth, p.Turns, worldOut)
				quitExecution(c, turn)
			}
		default:
			if quit {
				break
			}
			if p.Threads == 1 {
				worldOut = updateBoard(0, p.ImageHeight, worldIn, p)
			} else {
				out := make([]chan [][]uint8, p.Threads)

				for i := range out {
					out[i] = make(chan [][]uint8)
				}

				SmallHeight := p.ImageHeight / p.Threads
				BigHeight := SmallHeight + 1
				counter := 0

				for i := 0; i < p.ImageHeight%p.Threads; i++ {
					go worker(i*BigHeight, (i+1)*BigHeight, worldIn, out[i], p)
					counter++
				}

				start := (counter) * BigHeight
				end := start + SmallHeight

				for j := p.ImageHeight % p.Threads; j < p.Threads; j++ {
					go worker(start, end, worldIn, out[j], p)
					start = start + SmallHeight
					end = end + SmallHeight
				}

				worldOut = makeMatrix(0, 0)

				for i := 0; i < p.Threads; i++ {
					part := <-out[i]
					worldOut = append(worldOut, part...)
				}
			}

			turn++

			// check which cells have changed, and send CellFlipped event
			for row := 0; row < p.ImageHeight; row++ {
				for col := 0; col < p.ImageWidth; col++ {
					if worldOut[row][col] != worldIn[row][col] {
						c.events <- CellFlipped{turn, util.Cell{X: col, Y: row}}
					}
				}
			}

			// worldIn = worldOut before you move onto the next iteration
			for row := 0; row < p.ImageHeight; row++ {
				for col := 0; col < p.ImageWidth; col++ {
					worldIn[row][col] = worldOut[row][col]
				}
			}

			c.events <- TurnComplete{turn}
		}
	}

	// count final worldOut's state
	cells := calcAliveCells(worldIn, p.ImageHeight, p.ImageWidth)

	// OUTPUT operations
	saveWorldAsImage(c, name, p.ImageHeight, p.ImageWidth, turn, worldIn)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{p.Turns, cells}

	quitExecution(c, turn)
}
