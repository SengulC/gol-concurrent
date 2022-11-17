package gol

import (
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
	for i := range worldOut {
		worldOut[i] = worldIn[i]
	}

	turn := 0
	max := p.ImageHeight - 1
	// TODO: Execute all turns of the Game of Life.
	for turn < p.Turns {
		// worldOut = worldIn, at the start of every turn.
		for i := range worldOut {
			worldOut[i] = worldIn[i]
		}

		for row := 0; row < max; row++ {
			for col := 0; col < max; col++ {
				// CURRENT ELEMENT AND ITS NEIGHBOR COUNT RESET
				element := worldIn[row][col]
				counter := 0

				// iterate through all neighbors of given element
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						// creates 3x3 matrix w element as centerpiece, but centerpiece is included as well ofc.
						nRow := (row + dx + max) % max
						nCol := (col + dy + max) % max
						// increment counter if given neighbor is alive
						if worldIn[nRow][nCol] == 255 {
							counter++
							// fmt.Println(counter)
						}
					}
				}

				// if element is alive exclude it from the 3x3 matrix counter
				if element == 255 {
					counter--
				}

				// if element dead
				if element == 0 {
					if counter == 3 {
						worldOut[row][col] = 255
					}
				} else {
					// if element alive
					if counter < 2 {
						worldOut[row][col] = 0
					} else if counter > 3 {
						worldOut[row][col] = 0
					}
				}
			}
		}
		for i := range worldIn {
			worldIn[i] = worldOut[i]
		}
		turn++
	}

	// count final worldOut's state
	max = p.ImageHeight
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
