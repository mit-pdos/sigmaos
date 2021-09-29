#ifndef MATRIX_H
#define MATRIX_H

struct matrix {
	int m;
	int n;
  double **v;
};

struct matrix * alloc_matrix(int m, int n) {
	struct matrix *M = (struct matrix *) malloc(sizeof(struct matrix));
	M->m = m;
	M->n = n;
	M->v = (double **) malloc(M->m * sizeof(double *));
	for (int i = 0; i < M->m; ++i) {
		M->v[i] = (double *) malloc(M->n * sizeof(double));
	}
	return M;
}

void fill_random_non_zero(struct matrix *M) {
	for (int i = 0; i < M->m; ++i) {
		for (int j = 0; j < M->n; ++j) {
			M->v[i][j] = (double) rand() + 1.0;
		}
	}
}

void fill_zero(struct matrix *M) {
	for (int i = 0; i < M->m; ++i) {
		for (int j = 0; j < M->n; ++j) {
			M->v[i][j] = 0.0;
		}
	}
}

void mult(struct matrix *A, struct matrix *B, struct matrix *C) {
	if (A->n != B->m || C->m != A->m || C->n != B->n) {
		printf("Error in dimensions!\n");
	}
	for (int i = 0; i < A->m; ++i) {
		for (int j = 0; j < B->n; ++j) {
			C->v[i][j] = 0.0;
			for (int k = 0; k < A->n; ++k) {
				C->v[i][j] += A->v[i][k] * B->v[k][j];
			}
		}
	}
}

#endif
