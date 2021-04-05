#include <aws/lambda-runtime/runtime.h>
#include <aws/core/utils/json/JsonSerializer.h>
#include <aws/core/utils/memory/stl/SimpleStringStream.h>
#include <sys/time.h>
#include <string.h>

#include "matrix.h"

using namespace aws::lambda_runtime;

invocation_response my_handler(invocation_request const& request)
{

    using namespace Aws::Utils::Json;

    JsonValue json(request.payload);
    if (!json.WasParseSuccessful()) {
        return invocation_response::failure("Failed to parse input JSON", "InvalidJSON");
    }

    auto v = json.View();
    Aws::SimpleStringStream ss;

    struct timeval start, end;
    bool baseline = false;
    int its = 0;
    int dim = 0;

    if (v.ValueExists("body") && v.GetObject("body").IsString()) {
        auto body = v.GetString("body");
        JsonValue body_json(body);

        if (body_json.WasParseSuccessful()) {
            auto body_v = body_json.View();
            if (body_v.ValueExists("baseline") && body_v.GetObject("baseline").IsBool()) {
              baseline = body_v.GetBool("baseline");
            }
        } else {
          ss << "Error parsing body" << std::endl;
        }
    }

    if (v.ValueExists("queryStringParameters")) {
        auto query_params = v.GetObject("queryStringParameters");
        if (query_params.ValueExists("its") && query_params.GetObject("its").IsString()) {
          its = atoi(query_params.GetString("its").c_str());
        }
        if (query_params.ValueExists("dim") && query_params.GetObject("dim").IsString()) {
          dim = atoi(query_params.GetString("dim").c_str());
        }
    }

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
      ss << "Total elapsed computation time: " << elapsed << " usec(s)" << std::endl;
      ss << "Average computation time: " <<  ((double) elapsed) / ((double) its) << " usec(s)" << std::endl;
      ss << "Total elapsed setup time: " << 0.0 << " usec(s)" << std::endl;
    }

    JsonValue resp;
    resp.WithString("message", ss.str());

    return invocation_response::success(resp.View().WriteCompact(), "application/json");
}

int main()
{
    run_handler(my_handler);
    return 0;
}
