#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#include "matrix.h"

int main(int argc, char** argv) {
	if (argc < 4) {
		printf("Usage: spinner pid dim its <native/baseline>\n");
		exit(1);
	}

  struct timeval start, end;
	int dim = atoi(argv[2]);
	int its = atoi(argv[3]);
  int baseline = argc == 5 && strcmp("baseline", argv[4]) == 0;
	struct matrix *A = alloc_matrix(dim, dim);
	struct matrix *B = alloc_matrix(dim, dim);
	struct matrix *C = alloc_matrix(dim, dim);

	fill_random_non_zero(A);
	fill_random_non_zero(B);
	fill_zero(C);

  // If we're gathering a baseline...
  if (baseline) {
    gettimeofday(&start, NULL);
  }

	for (int i = 0; i < its; ++i) {
		if (i % 3 == 0) {
			mult(A, B, C);
		} else {
			mult(B, A, C);
		}
	}

  // If we're gathering a baseline...
  if (baseline) {
    gettimeofday(&end, NULL);
    int elapsed = (end.tv_sec - start.tv_sec) * 1000 * 1000 + (end.tv_usec - start.tv_usec);
    printf("Total elapsed computation time: %d usec(s)\n", elapsed);
    printf("Average computation time: %f usec(s)\n", ((double) elapsed) / ((double) its));
    printf("Total elapsed setup time: %f usec(s)\n", 0.0);
  }
}
