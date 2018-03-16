// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#define Py_LIMITED_API
#include <Python.h>
#include "api.h"

static PyMethodDef Agent_methods[] = {
    {"shutdown", (PyCFunction)AgentShutdown, METH_NOARGS, "Destroy an agent."},
    {"add_report", (PyCFunction)AgentAddReport, METH_O, "Add a usage report."},
    {"get_status", (PyCFunction)AgentGetStatus, METH_NOARGS, "Get agent status."},
    {NULL}
};

static PyTypeObject Agent_type = {
    PyVarObject_HEAD_INIT(NULL, 0)
    "ubbagent.Agent",             // tp_name
    sizeof(Agent),                // tp_basicsize
    0,                            // tp_itemsize
    (destructor)AgentDealloc,     // tp_dealloc
    0,                            // tp_print
    0,                            // tp_getattr
    0,                            // tp_setattr
    0,                            // tp_reserved
    0,                            // tp_repr
    0,                            // tp_as_number
    0,                            // tp_as_sequence
    0,                            // tp_as_mapping
    0,                            // tp_hash
    0,                            // tp_call
    0,                            // tp_str
    0,                            // tp_getattro
    0,                            // tp_setattro
    0,                            // tp_as_buffer
    Py_TPFLAGS_DEFAULT,           // tp_flags
    "Agent objects",              // tp_doc
    0,                            // tp_traverse
    0,                            // tp_clear
    0,                            // tp_richcompare
    0,                            // tp_weaklistoffset
    0,                            // tp_iter
    0,                            // tp_iternext
    Agent_methods,                // tp_methods
    0,                            // tp_members
    0,                            // tp_getset
    0,                            // tp_base
    0,                            // tp_dict
    0,                            // tp_descr_get
    0,                            // tp_descr_set
    0,                            // tp_dictoffset
    (initproc)AgentInit,          // tp_init
    0,                            // tp_alloc
    PyType_GenericNew,            // tp_new
};

PyMODINIT_FUNC
initubbagent(void) {
  PyObject* m = Py_InitModule("ubbagent", 0);
  if (m == NULL) {
    return;
  }

  if (PyType_Ready(&Agent_type) < 0) {
    Py_DECREF(m);
    return;
  }

  Py_INCREF(&Agent_type);
  PyModule_AddObject(m, "Agent", (PyObject *)&Agent_type);

  AgentError = PyErr_NewException("ubbagent.AgentError", NULL, NULL);
  Py_INCREF(AgentError);
  PyModule_AddObject(m, "AgentError", AgentError);
}
