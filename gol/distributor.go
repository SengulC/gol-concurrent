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

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func differenceInFlippedCells(height, width int, worldIn, worldOut [][]byte) int {
	count := 0
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if worldOut[row][col] != worldIn[row][col] {
				count++
			}
		}
	}
	return count
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

func worker(startY, endY, currentThread, currentTurn int, worldIn [][]byte, out chan<- [][]uint8, p Params, events chan<- Event) {
	boardSeg := updateBoard(startY, endY, currentThread, currentTurn, worldIn, p, events)
	out <- boardSeg
}

// UpdateBoard updates and returns a single iteration of GOL
func updateBoard(startY, endY, currentThread, currentTurn int, worldIn [][]byte, p Params, events chan<- Event) [][]byte {
	segHeight := endY - startY
	//if currentThread+1 == p.Threads && p.Threads%2 != 0 && currentThread != 0 {
	//	segHeight++
	//}

	// worldOut = 0
	worldOut := make([][]byte, segHeight)
	for row := 0; row < segHeight; row++ {
		worldOut[row] = make([]byte, p.ImageWidth)
		for col := 0; col < p.ImageWidth; col++ {
			worldOut[row][col] = 0
		}
	}

	//endY2 := endY
	//// if it's the LAST thread, and number of threads is ODD, but it is NOT thread 0
	//if currentThread+1 == p.Threads && p.Threads%2 != 0 && currentThread != 0 {
	//	endY2 = endY + 1
	//}

	for row := startY; row < endY; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			// CURRENT ELEMENT AND ITS NEIGHBOR COUNT RESET
			element := worldIn[row][col]
			counter := 0

			// iterate through all neighbors of given element
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					// creates 3x3 matrix w element as centerpiece, but centerpiece is included as well ofc.
					nRow := (row + dx + p.ImageHeight) % p.ImageHeight
					nCol := (col + dy + p.ImageWidth) % p.ImageWidth
					// increment counter if given neighbor is alive
					if worldIn[nRow][nCol] == 255 {
						counter++
					}
				}
			}

			// if element is alive exclude it from the 3x3 matrix counter
			if element == 255 {
				counter--
			}

			superRow := row - startY

			// if element dead
			if element == 0 {
				if counter == 3 {
					worldOut[superRow][col] = 255
					// CHANGE COL ROW ORDER
					//events <- CellFlipped{currentTurn, util.Cell{X: superRow, Y: col}}
				} else {
					worldOut[superRow][col] = 0
				}
			} else {
				// if element alive
				if counter < 2 {
					worldOut[superRow][col] = 0
					//events <- CellFlipped{currentTurn, util.Cell{X: superRow, Y: col}}
				} else if counter > 3 {
					worldOut[superRow][col] = 0
					//events <- CellFlipped{currentTurn, util.Cell{X: superRow, Y: col}}
				} else {
					worldOut[superRow][col] = 255
				}
			}
		}
	}
	return worldOut
}

func pauseLoop(kP <-chan rune, pause chan bool) {
	for {
		k := <-kP
		if k == 'p' {
			pause <- true
			break
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.
	name := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- name

	worldIn := makeMatrix(p.ImageHeight, p.ImageWidth)
	// get image byte by byte and store in: worldIn
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			worldIn[row][col] = <-c.ioInput
			if worldIn[row][col] != 0 {
				c.events <- CellFlipped{0, util.Cell{col, row}}
			}
		}
	}

	fmt.Println("loaded image w alive cell count:", calcAliveCellCount(p.ImageHeight, p.ImageWidth, worldIn))

	// calculate alive cells of worldIn and pass a flipped event for each
	//aliveCells := calcAliveCells(worldIn, p.ImageHeight, p.ImageWidth)
	//for i := range aliveCells {
	//	c.events <- CellFlipped{0, aliveCells[i]}
	//}

	//c.events <- TurnComplete{1}

	worldOut := makeMatrix(p.ImageHeight, p.ImageWidth)
	// worldOut = worldIn
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			worldOut[row][col] = worldIn[row][col]
		}
	}

	timeOver := time.NewTicker(2 * time.Second)

	turn := 0
	var key rune
	pause := make(chan bool, 1)
	quit := false

	//var worldOut1 [][]byte
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
				//pause
				fmt.Println("Paused. Current turn:", turn)
				go pauseLoop(c.keyPresses, pause)
				_ = <-pause
				fmt.Println("Continuing.")
			case 's':
				//save
				fmt.Println("Saving")
				c.ioCommand <- ioOutput
				c.ioFilename <- name + "x" + strconv.Itoa(p.Turns)
				for row := 0; row < p.ImageHeight; row++ {
					for col := 0; col < p.ImageWidth; col++ {
						c.ioOutput <- worldOut[row][col]
					}
				}
			case 'q':
				//quit
				quit = true
				fmt.Println("Quitting")
				c.ioCommand <- ioOutput
				c.ioFilename <- name + "x" + strconv.Itoa(p.Turns)
				for row := 0; row < p.ImageHeight; row++ {
					for col := 0; col < p.ImageWidth; col++ {
						c.ioOutput <- worldOut[row][col]
					}
				}
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- StateChange{turn, Quitting}
				close(c.events)
			}
		default:
			if quit {
				break
			}
			if p.Threads == 1 {
				worldOut = updateBoard(0, p.ImageHeight, 0, turn, worldIn, p, c.events)
			} else {
				workerHeight := p.ImageHeight / p.Threads
				out := make([]chan [][]uint8, p.Threads)

				for i := range out {
					out[i] = make(chan [][]uint8)
				}

				// small height = height / threads
				// big height = small height + 1
				// do one of them height%threads times, n the other the rest of the time

				for i := 0; i < p.Threads; i++ {
					endY := (i + 1) * workerHeight
					if i == p.Threads-1 {
						endY = p.ImageHeight
					}
					go worker(i*workerHeight, endY, i, turn, worldIn, out[i], p, c.events)
					//fmt.Println("worker heights", endY - (i * workerHeight))
				}

				worldOut = makeMatrix(0, 0)

				for i := 0; i < p.Threads; i++ {
					part := <-out[i]
					worldOut = append(worldOut, part...)
				}
			}

			turn++

			// check which cells have changed
			//fmt.Println("turn", turn, "alive cell count:", calcAliveCellCount(p.ImageHeight, p.ImageWidth, worldOut))
			//fmt.Println("difference in flipped cells:", differenceInFlippedCells(p.ImageHeight, p.ImageWidth, worldIn, worldOut))
			for row := 0; row < p.ImageHeight; row++ {
				for col := 0; col < p.ImageWidth; col++ {
					if p.Threads != 1 {
						if worldOut[row][col] != worldIn[row][col] {
							//c.events <- CellFlipped{turn, util.Cell{X: row, Y: col}}
						}
					} else {
						if worldOut[row][col] != worldIn[row][col] {
							// NOTE: print out wO and wI elems and state when theyre changed
							// CALCALIVECELLSCOUNT(WO) - CACC(WI) = NUM. RUN FOR LOOP NUM TIMES
							c.events <- CellFlipped{turn, util.Cell{X: col, Y: row}}
						}
					}
				}
			}

			// worldIn = worldOut before you move onto the next iteration
			for row := 0; row < p.ImageHeight; row++ {
				for col := 0; col < p.ImageWidth; col++ {
					if p.Threads != 1 {
						worldIn[row][col] = worldOut[row][col]
					} else {
						worldIn[row][col] = worldOut[row][col]
					}
				}
			}

			c.events <- TurnComplete{turn}
		}
	}

	// count final worldOut's state
	cells := calcAliveCells(worldIn, p.ImageHeight, p.ImageWidth)

	// save to output file
	c.ioCommand <- ioOutput
	c.ioFilename <- name + "x" + strconv.Itoa(p.Turns)
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			c.ioOutput <- worldIn[row][col]
		}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	// pass it down events channel (list of alive cells)
	c.events <- FinalTurnComplete{p.Turns, cells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
