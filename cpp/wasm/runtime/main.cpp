#include <wasmedge/wasmedge.h>
#include <proxy/sigmap/sigmap.h>
#include <iostream>

std::shared_ptr<sigmaos::proxy::sigmap::Clnt> clnt;

WasmEdge_Result Started(void *Data, 
    const WasmEdge_CallingFrameContext *CallFrameCxt, const WasmEdge_Value *In,
    WasmEdge_Value *Out) {

    std::cout << "Started() called from WASM" << std::endl;
    auto res = clnt -> Started();
    
    if (res.has_value()) {
        std::cout << "Successfully called sigmap Started()" << std::endl;
    } else {
        std::cerr << "Error calling sigmap Started(): " << res.error().String() << std::endl;
    }
    Out[0] = WasmEdge_ValueGenI32(123);
    return WasmEdge_Result_Success;
}

WasmEdge_Result Exited(void* Data,
    const WasmEdge_CallingFrameContext *CallFrameCtx,
    const WasmEdge_Value *In,
    WasmEdge_Value *Out){
        
    int32_t status = WasmEdge_ValueGetI32(In[0]);
    std::cout << "Exited() called with status: " << status << std::endl;
    return WasmEdge_Result_Success;
}

int main(int argc, char** argv) {

    clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

    WasmEdge_ConfigureContext* ConfCxt = WasmEdge_ConfigureCreate();
    WasmEdge_ConfigureAddHostRegistration(ConfCxt, WasmEdge_HostRegistration_Wasi);

    WasmEdge_VMContext* VMCxt = WasmEdge_VMCreate(ConfCxt, nullptr);

    WasmEdge_String ModuleName = WasmEdge_StringCreateByCString("env");
    WasmEdge_ModuleInstanceContext *HostModContext = WasmEdge_ModuleInstanceCreate(ModuleName);

    // host functions
    WasmEdge_ValType StartedReturns[] = {WasmEdge_ValTypeGenI32()};
    WasmEdge_FunctionTypeContext *StartedType = WasmEdge_FunctionTypeCreate(nullptr, 0, StartedReturns, 1);
    WasmEdge_FunctionInstanceContext *StartedFunc = WasmEdge_FunctionInstanceCreate(StartedType, Started, NULL, 0);
    WasmEdge_String StartedName = WasmEdge_StringCreateByCString("Started");
    WasmEdge_ModuleInstanceAddFunction(HostModContext, StartedName, StartedFunc);

    WasmEdge_ValType ExitedParams[] = {WasmEdge_ValTypeGenI32()};
    WasmEdge_FunctionTypeContext *ExitedType = WasmEdge_FunctionTypeCreate(ExitedParams, 1, nullptr, 0);
    WasmEdge_FunctionInstanceContext *ExitedFunc = WasmEdge_FunctionInstanceCreate(ExitedType, Exited, NULL, 0);
    WasmEdge_String ExitedName = WasmEdge_StringCreateByCString("Exited");
    WasmEdge_ModuleInstanceAddFunction(HostModContext, ExitedName, ExitedFunc);

    WasmEdge_VMRegisterModuleFromImport(VMCxt, HostModContext);

    WasmEdge_Value params[1] = {WasmEdge_ValueGenI32(2)};
    WasmEdge_Value returns[1];

    WasmEdge_String func_name = WasmEdge_StringCreateByCString("test");

    WasmEdge_Result res = WasmEdge_VMRunWasmFromFile(VMCxt, argv[1], func_name, params, 1, returns, 1);

    if(WasmEdge_ResultOK(res)) {
        std::cout << "Get result: " << WasmEdge_ValueGetI32(returns[0]) << std::endl;
    } else {
        std::cout << "Error message: " << WasmEdge_ResultGetMessage(res) << std::endl;
    }

    WasmEdge_VMDelete(VMCxt);
    WasmEdge_ConfigureDelete(ConfCxt);
    WasmEdge_StringDelete(func_name);
    return 0;
}