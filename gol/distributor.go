package gol

import (
	"fmt"
	"strconv"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func worker(startY, endY, currentThread int, worldIn [][]byte, out chan<- [][]uint8, p Params) {
	boardSeg := updateBoard(startY, endY, currentThread, worldIn, p)
	out <- boardSeg
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

// UpdateBoard updates and returns a single iteration of GOL
func updateBoard(startY, endY, currentThread int, worldIn [][]byte, p Params) [][]byte {
	segHeight := endY - startY
	if currentThread+1 == p.Threads && p.Threads%2 != 0 && currentThread != 0 {
		segHeight++
	}

	// worldOut = 0
	worldOut := make([][]byte, segHeight)
	for row := 0; row < segHeight; row++ {
		worldOut[row] = make([]byte, p.ImageWidth)
		for col := 0; col < p.ImageWidth; col++ {
			worldOut[row][col] = 0
		}
	}

	fmt.Println("just made worldOut, current thread:", currentThread)
	util.VisualiseMatrix(worldOut, p.ImageWidth, segHeight)

	//fmt.Println("made worldOut:", segHeight, "x", p.ImageWidth)

	endY2 := endY
	// if it's the LAST thread, and number of threads is ODD, but it is NOT thread 0
	if currentThread+1 == p.Threads && p.Threads%2 != 0 && currentThread != 0 {
		endY2 = endY + 1
	}

	for row := startY; row < endY2; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			// CURRENT ELEMENT AND ITS NEIGHBOR COUNT RESET
			element := worldIn[row][col]
			//fmt.Println("elem [r][c]:", row, col)
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

			superRow := row - (currentThread * (endY - startY))
			//fmt.Println("superrow=", superRow)

			// if element dead
			if element == 0 {
				if counter == 3 {
					worldOut[superRow][col] = 255
				} else {
					worldOut[superRow][col] = 0
				}
			} else {
				// if element alive
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

	fmt.Println("UPDATED worldOut")
	util.VisualiseMatrix(worldOut, p.ImageWidth, segHeight)

	return worldOut
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.
	name := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- name

	worldIn := make([][]byte, p.ImageHeight)
	for i := range worldIn {
		worldIn[i] = make([]byte, p.ImageWidth)
	}
	// get image byte by byte and store in: worldIn
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			worldIn[row][col] = <-c.ioInput
		}
	}

	// worldOut = worldIn
	worldOut := make([][]byte, p.ImageHeight)
	for row := 0; row < p.ImageHeight; row++ {
		worldOut[row] = make([]byte, p.ImageWidth)
		for col := 0; col < p.ImageWidth; col++ {
			worldOut[row][col] = worldIn[row][col]
		}
	}

	turn := 0
	// TODO: Execute all turns of the Game of Life.
	for turn < p.Turns {
		if p.Threads == 1 {
			worldOut = updateBoard(0, p.ImageHeight, 0, worldIn, p)
		} else {
			workerHeight := p.ImageHeight / p.Threads
			fmt.Println("p.Threads, workerHeight:")
			fmt.Println(p.Threads, workerHeight)
			out := make([]chan [][]uint8, p.Threads)

			for i := range out {
				out[i] = make(chan [][]uint8)
			}

			for i := 0; i < p.Threads; i++ {
				go worker(i*workerHeight, (i+1)*workerHeight, i, worldIn, out[i], p)
			}

			// call worker twice

			worldOut = makeMatrix(0, 0)

			for i := 0; i < p.Threads; i++ {
				part := <-out[i]
				worldOut = append(worldOut, part...)
			}
		}

		// worldIn = worldOut before you move onto the next iteration
		for row := 0; row < p.ImageHeight; row++ {
			for col := 0; col < p.ImageWidth; col++ {
				worldIn[row][col] = worldOut[row][col]
			}
		}
		turn++
	}

	// count final worldOut's state
	max := p.ImageHeight
	var count int
	var cells []util.Cell
	for row := 0; row < max; row++ {
		for col := 0; col < max; col++ {
			if worldOut[row][col] == 255 {
				c := util.Cell{X: col, Y: row}
				cells = append(cells, c)
				count++
			}
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
