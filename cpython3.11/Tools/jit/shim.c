#include "Python.h"

#include "pycore_ceval.h"
#include "pycore_frame.h"
#include "pycore_jit.h"

#include "jit.h"

_Py_CODEUNIT *
_JIT_ENTRY(_PyInterpreterFrame *frame, _PyStackRef *stack_pointer, PyThreadState *tstate)
{
    // This is subtle. The actual trace will return to us once it exits, so we
    // need to make sure that we stay alive until then. If our trace side-exits
    // into another trace, and this trace is then invalidated, the code for
    // *this function* will be freed and we'll crash upon return:
    PyAPI_DATA(void) _JIT_EXECUTOR;
    PyObject *executor = (PyObject *)(uintptr_t)&_JIT_EXECUTOR;
    Py_INCREF(executor);
    // Note that this is *not* a tail call:
    PyAPI_DATA(void) _JIT_CONTINUE;
    _Py_CODEUNIT *target = ((jit_func_preserve_none)&_JIT_CONTINUE)(frame, stack_pointer, tstate);
    Py_SETREF(tstate->previous_executor, executor);
    return target;
}
