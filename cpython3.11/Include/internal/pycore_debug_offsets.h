#ifndef Py_INTERNAL_DEBUG_OFFSETS_H
#define Py_INTERNAL_DEBUG_OFFSETS_H
#ifdef __cplusplus
extern "C" {
#endif

#ifndef Py_BUILD_CORE
#  error "this header requires Py_BUILD_CORE define"
#endif


#define _Py_Debug_Cookie "xdebugpy"

#ifdef Py_GIL_DISABLED
# define _Py_Debug_gilruntimestate_enabled offsetof(struct _gil_runtime_state, enabled)
# define _Py_Debug_Free_Threaded 1
#else
# define _Py_Debug_gilruntimestate_enabled 0
# define _Py_Debug_Free_Threaded 0
#endif


typedef struct _Py_DebugOffsets {
    char cookie[8];
    uint64_t version;
    uint64_t free_threaded;
    // Runtime state offset;
    struct _runtime_state {
        uint64_t size;
        uint64_t finalizing;
        uint64_t interpreters_head;
    } runtime_state;

    // Interpreter state offset;
    struct _interpreter_state {
        uint64_t size;
        uint64_t id;
        uint64_t next;
        uint64_t threads_head;
        uint64_t gc;
        uint64_t imports_modules;
        uint64_t sysdict;
        uint64_t builtins;
        uint64_t ceval_gil;
        uint64_t gil_runtime_state;
        uint64_t gil_runtime_state_enabled;
        uint64_t gil_runtime_state_locked;
        uint64_t gil_runtime_state_holder;
    } interpreter_state;

    // Thread state offset;
    struct _thread_state{
        uint64_t size;
        uint64_t prev;
        uint64_t next;
        uint64_t interp;
        uint64_t current_frame;
        uint64_t thread_id;
        uint64_t native_thread_id;
        uint64_t datastack_chunk;
        uint64_t status;
    } thread_state;

    // InterpreterFrame offset;
    struct _interpreter_frame {
        uint64_t size;
        uint64_t previous;
        uint64_t executable;
        uint64_t instr_ptr;
        uint64_t localsplus;
        uint64_t owner;
    } interpreter_frame;

    // Code object offset;
    struct _code_object {
        uint64_t size;
        uint64_t filename;
        uint64_t name;
        uint64_t qualname;
        uint64_t linetable;
        uint64_t firstlineno;
        uint64_t argcount;
        uint64_t localsplusnames;
        uint64_t localspluskinds;
        uint64_t co_code_adaptive;
    } code_object;

    // PyObject offset;
    struct _pyobject {
        uint64_t size;
        uint64_t ob_type;
    } pyobject;

    // PyTypeObject object offset;
    struct _type_object {
        uint64_t size;
        uint64_t tp_name;
        uint64_t tp_repr;
        uint64_t tp_flags;
    } type_object;

    // PyTuple object offset;
    struct _tuple_object {
        uint64_t size;
        uint64_t ob_item;
        uint64_t ob_size;
    } tuple_object;

    // PyList object offset;
    struct _list_object {
        uint64_t size;
        uint64_t ob_item;
        uint64_t ob_size;
    } list_object;

    // PyDict object offset;
    struct _dict_object {
        uint64_t size;
        uint64_t ma_keys;
        uint64_t ma_values;
    } dict_object;

    // PyFloat object offset;
    struct _float_object {
        uint64_t size;
        uint64_t ob_fval;
    } float_object;

    // PyLong object offset;
    struct _long_object {
        uint64_t size;
        uint64_t lv_tag;
        uint64_t ob_digit;
    } long_object;

    // PyBytes object offset;
    struct _bytes_object {
        uint64_t size;
        uint64_t ob_size;
        uint64_t ob_sval;
    } bytes_object;

    // Unicode object offset;
    struct _unicode_object {
        uint64_t size;
        uint64_t state;
        uint64_t length;
        uint64_t asciiobject_size;
    } unicode_object;

    // GC runtime state offset;
    struct _gc {
        uint64_t size;
        uint64_t collecting;
    } gc;
} _Py_DebugOffsets;


#define _Py_DebugOffsets_INIT(debug_cookie) { \
    .cookie = debug_cookie, \
    .version = PY_VERSION_HEX, \
    .free_threaded = _Py_Debug_Free_Threaded, \
    .runtime_state = { \
        .size = sizeof(_PyRuntimeState), \
        .finalizing = offsetof(_PyRuntimeState, _finalizing), \
        .interpreters_head = offsetof(_PyRuntimeState, interpreters.head), \
    }, \
    .interpreter_state = { \
        .size = sizeof(PyInterpreterState), \
        .id = offsetof(PyInterpreterState, id), \
        .next = offsetof(PyInterpreterState, next), \
        .threads_head = offsetof(PyInterpreterState, threads.head), \
        .gc = offsetof(PyInterpreterState, gc), \
        .imports_modules = offsetof(PyInterpreterState, imports.modules), \
        .sysdict = offsetof(PyInterpreterState, sysdict), \
        .builtins = offsetof(PyInterpreterState, builtins), \
        .ceval_gil = offsetof(PyInterpreterState, ceval.gil), \
        .gil_runtime_state = offsetof(PyInterpreterState, _gil), \
        .gil_runtime_state_enabled = _Py_Debug_gilruntimestate_enabled, \
        .gil_runtime_state_locked = offsetof(PyInterpreterState, _gil.locked), \
        .gil_runtime_state_holder = offsetof(PyInterpreterState, _gil.last_holder), \
    }, \
    .thread_state = { \
        .size = sizeof(PyThreadState), \
        .prev = offsetof(PyThreadState, prev), \
        .next = offsetof(PyThreadState, next), \
        .interp = offsetof(PyThreadState, interp), \
        .current_frame = offsetof(PyThreadState, current_frame), \
        .thread_id = offsetof(PyThreadState, thread_id), \
        .native_thread_id = offsetof(PyThreadState, native_thread_id), \
        .datastack_chunk = offsetof(PyThreadState, datastack_chunk), \
        .status = offsetof(PyThreadState, _status), \
    }, \
    .interpreter_frame = { \
        .size = sizeof(_PyInterpreterFrame), \
        .previous = offsetof(_PyInterpreterFrame, previous), \
        .executable = offsetof(_PyInterpreterFrame, f_executable), \
        .instr_ptr = offsetof(_PyInterpreterFrame, instr_ptr), \
        .localsplus = offsetof(_PyInterpreterFrame, localsplus), \
        .owner = offsetof(_PyInterpreterFrame, owner), \
    }, \
    .code_object = { \
        .size = sizeof(PyCodeObject), \
        .filename = offsetof(PyCodeObject, co_filename), \
        .name = offsetof(PyCodeObject, co_name), \
        .qualname = offsetof(PyCodeObject, co_qualname), \
        .linetable = offsetof(PyCodeObject, co_linetable), \
        .firstlineno = offsetof(PyCodeObject, co_firstlineno), \
        .argcount = offsetof(PyCodeObject, co_argcount), \
        .localsplusnames = offsetof(PyCodeObject, co_localsplusnames), \
        .localspluskinds = offsetof(PyCodeObject, co_localspluskinds), \
        .co_code_adaptive = offsetof(PyCodeObject, co_code_adaptive), \
    }, \
    .pyobject = { \
        .size = sizeof(PyObject), \
        .ob_type = offsetof(PyObject, ob_type), \
    }, \
    .type_object = { \
        .size = sizeof(PyTypeObject), \
        .tp_name = offsetof(PyTypeObject, tp_name), \
        .tp_repr = offsetof(PyTypeObject, tp_repr), \
        .tp_flags = offsetof(PyTypeObject, tp_flags), \
    }, \
    .tuple_object = { \
        .size = sizeof(PyTupleObject), \
        .ob_item = offsetof(PyTupleObject, ob_item), \
        .ob_size = offsetof(PyTupleObject, ob_base.ob_size), \
    }, \
    .list_object = { \
        .size = sizeof(PyListObject), \
        .ob_item = offsetof(PyListObject, ob_item), \
        .ob_size = offsetof(PyListObject, ob_base.ob_size), \
    }, \
    .dict_object = { \
        .size = sizeof(PyDictObject), \
        .ma_keys = offsetof(PyDictObject, ma_keys), \
        .ma_values = offsetof(PyDictObject, ma_values), \
    }, \
    .float_object = { \
        .size = sizeof(PyFloatObject), \
        .ob_fval = offsetof(PyFloatObject, ob_fval), \
    }, \
    .long_object = { \
        .size = sizeof(PyLongObject), \
        .lv_tag = offsetof(PyLongObject, long_value.lv_tag), \
        .ob_digit = offsetof(PyLongObject, long_value.ob_digit), \
    }, \
    .bytes_object = { \
        .size = sizeof(PyBytesObject), \
        .ob_size = offsetof(PyBytesObject, ob_base.ob_size), \
        .ob_sval = offsetof(PyBytesObject, ob_sval), \
    }, \
    .unicode_object = { \
        .size = sizeof(PyUnicodeObject), \
        .state = offsetof(PyUnicodeObject, _base._base.state), \
        .length = offsetof(PyUnicodeObject, _base._base.length), \
        .asciiobject_size = sizeof(PyASCIIObject), \
    }, \
    .gc = { \
        .size = sizeof(struct _gc_runtime_state), \
        .collecting = offsetof(struct _gc_runtime_state, collecting), \
    }, \
}


#ifdef __cplusplus
}
#endif
#endif /* !Py_INTERNAL_DEBUG_OFFSETS_H */
