webpackHotUpdate_N_E("pages/index",{

/***/ "./pages/index.tsx":
/*!*************************!*\
  !*** ./pages/index.tsx ***!
  \*************************/
/*! exports provided: default */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* WEBPACK VAR INJECTION */(function(module) {/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "default", function() { return Home; });
/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-dev-runtime */ "./node_modules/react/jsx-dev-runtime.js");
/* harmony import */ var react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/classCallCheck */ "./node_modules/@babel/runtime/helpers/esm/classCallCheck.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/createClass */ "./node_modules/@babel/runtime/helpers/esm/createClass.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_assertThisInitialized__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/assertThisInitialized */ "./node_modules/@babel/runtime/helpers/esm/assertThisInitialized.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/inherits */ "./node_modules/@babel/runtime/helpers/esm/inherits.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn */ "./node_modules/@babel/runtime/helpers/esm/possibleConstructorReturn.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/getPrototypeOf */ "./node_modules/@babel/runtime/helpers/esm/getPrototypeOf.js");
/* harmony import */ var C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_defineProperty__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! ./node_modules/@babel/runtime/helpers/esm/defineProperty */ "./node_modules/@babel/runtime/helpers/esm/defineProperty.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__(/*! react */ "./node_modules/react/index.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_8___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_8__);
/* harmony import */ var _components_Clock__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__(/*! ../components/Clock */ "./components/Clock.tsx");
/* harmony import */ var _components_Message__WEBPACK_IMPORTED_MODULE_10__ = __webpack_require__(/*! ../components/Message */ "./components/Message.tsx");
/* harmony import */ var _components_DefaultRoom__WEBPACK_IMPORTED_MODULE_11__ = __webpack_require__(/*! ../components/DefaultRoom */ "./components/DefaultRoom.tsx");
/* harmony import */ var _components_StandingRoom__WEBPACK_IMPORTED_MODULE_12__ = __webpack_require__(/*! ../components/StandingRoom */ "./components/StandingRoom.tsx");
/* harmony import */ var _lib_fetcher__WEBPACK_IMPORTED_MODULE_13__ = __webpack_require__(/*! ../lib/fetcher */ "./lib/fetcher.ts");








var _jsxFileName = "C:\\Users\\momom\\Documents\\GitHub\\youtube-study-space\\youtube-monitor\\pages\\index.tsx";

function _createSuper(Derived) { var hasNativeReflectConstruct = _isNativeReflectConstruct(); return function _createSuperInternal() { var Super = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_6__["default"])(Derived), result; if (hasNativeReflectConstruct) { var NewTarget = Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_getPrototypeOf__WEBPACK_IMPORTED_MODULE_6__["default"])(this).constructor; result = Reflect.construct(Super, arguments, NewTarget); } else { result = Super.apply(this, arguments); } return Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_possibleConstructorReturn__WEBPACK_IMPORTED_MODULE_5__["default"])(this, result); }; }

function _isNativeReflectConstruct() { if (typeof Reflect === "undefined" || !Reflect.construct) return false; if (Reflect.construct.sham) return false; if (typeof Proxy === "function") return true; try { Date.prototype.toString.call(Reflect.construct(Date, [], function () {})); return true; } catch (e) { return false; } }








var Home = /*#__PURE__*/function (_React$Component) {
  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_inherits__WEBPACK_IMPORTED_MODULE_4__["default"])(Home, _React$Component);

  var _super = _createSuper(Home);

  function Home(props) {
    var _this;

    Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_classCallCheck__WEBPACK_IMPORTED_MODULE_1__["default"])(this, Home);

    _this = _super.call(this, props);

    Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_defineProperty__WEBPACK_IMPORTED_MODULE_7__["default"])(Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_assertThisInitialized__WEBPACK_IMPORTED_MODULE_3__["default"])(_this), "intervalId", void 0);

    _this.state = {
      layout: null,
      default_room_state: null,
      no_seat_room_state: null
    };
    return _this;
  }

  Object(C_Users_momom_Documents_GitHub_youtube_study_space_youtube_monitor_node_modules_babel_runtime_helpers_esm_createClass__WEBPACK_IMPORTED_MODULE_2__["default"])(Home, [{
    key: "componentDidMount",
    value: function componentDidMount() {
      var component = this;
      this.intervalId = setInterval(function () {
        Object(_lib_fetcher__WEBPACK_IMPORTED_MODULE_13__["default"])("https://taa4p9klha.execute-api.ap-northeast-1.amazonaws.com/rooms_state").then(function (r) {
          r.default_room.seats.forEach(function (item) {
            return console.log(item.seat_id, item.user_display_name);
          });
          console.log("fetch完了");
          component.setState({
            layout: r.default_room_layout,
            default_room_state: r.default_room,
            no_seat_room_state: r.no_seat_room
          });
        })["catch"](function (err) {
          return console.error(err);
        });
      }, 1500);
    }
  }, {
    key: "componentWillUnmount",
    value: function componentWillUnmount() {
      if (this.intervalId) {
        clearInterval(this.intervalId);
      }
    }
  }, {
    key: "render",
    value: function render() {
      return /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])("div", {
        style: {
          height: 1080,
          width: 1920,
          margin: 0,
          position: "relative"
        },
        children: [/*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])(_components_Clock__WEBPACK_IMPORTED_MODULE_9__["default"], {}, void 0, false, {
          fileName: _jsxFileName,
          lineNumber: 58,
          columnNumber: 9
        }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])(_components_Message__WEBPACK_IMPORTED_MODULE_10__["default"], {
          default_room_state: this.state.default_room_state,
          no_seat_room_state: this.state.no_seat_room_state
        }, void 0, false, {
          fileName: _jsxFileName,
          lineNumber: 59,
          columnNumber: 9
        }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])(_components_DefaultRoom__WEBPACK_IMPORTED_MODULE_11__["default"], {
          layout: this.state.layout,
          roomState: this.state.default_room_state
        }, void 0, false, {
          fileName: _jsxFileName,
          lineNumber: 63,
          columnNumber: 9
        }, this), /*#__PURE__*/Object(react_jsx_dev_runtime__WEBPACK_IMPORTED_MODULE_0__["jsxDEV"])(_components_StandingRoom__WEBPACK_IMPORTED_MODULE_12__["default"], {
          no_seat_room_state: this.state.no_seat_room_state
        }, void 0, false, {
          fileName: _jsxFileName,
          lineNumber: 67,
          columnNumber: 9
        }, this)]
      }, void 0, true, {
        fileName: _jsxFileName,
        lineNumber: 50,
        columnNumber: 7
      }, this);
    }
  }]);

  return Home;
}(react__WEBPACK_IMPORTED_MODULE_8___default.a.Component);



;
    var _a, _b;
    // Legacy CSS implementations will `eval` browser code in a Node.js context
    // to extract CSS. For backwards compatibility, we need to check we're in a
    // browser context before continuing.
    if (typeof self !== 'undefined' &&
        // AMP / No-JS mode does not inject these helpers:
        '$RefreshHelpers$' in self) {
        var currentExports = module.__proto__.exports;
        var prevExports = (_b = (_a = module.hot.data) === null || _a === void 0 ? void 0 : _a.prevExports) !== null && _b !== void 0 ? _b : null;
        // This cannot happen in MainTemplate because the exports mismatch between
        // templating and execution.
        self.$RefreshHelpers$.registerExportsForReactRefresh(currentExports, module.i);
        // A module can be accepted automatically based on its exports, e.g. when
        // it is a Refresh Boundary.
        if (self.$RefreshHelpers$.isReactRefreshBoundary(currentExports)) {
            // Save the previous exports on update so we can compare the boundary
            // signatures.
            module.hot.dispose(function (data) {
                data.prevExports = currentExports;
            });
            // Unconditionally accept an update to this module, we'll check if it's
            // still a Refresh Boundary later.
            module.hot.accept();
            // This field is set when the previous version of this module was a
            // Refresh Boundary, letting us know we need to check for invalidation or
            // enqueue an update.
            if (prevExports !== null) {
                // A boundary can become ineligible if its exports are incompatible
                // with the previous exports.
                //
                // For example, if you add/remove/change exports, we'll want to
                // re-execute the importing modules, and force those components to
                // re-render. Similarly, if you convert a class component to a
                // function, we want to invalidate the boundary.
                if (self.$RefreshHelpers$.shouldInvalidateReactRefreshBoundary(prevExports, currentExports)) {
                    module.hot.invalidate();
                }
                else {
                    self.$RefreshHelpers$.scheduleUpdate();
                }
            }
        }
        else {
            // Since we just executed the code for the module, it's possible that the
            // new exports made it ineligible for being a boundary.
            // We only care about the case when we were _previously_ a boundary,
            // because we already accepted this update (accidental side effect).
            var isNoLongerABoundary = prevExports !== null;
            if (isNoLongerABoundary) {
                module.hot.invalidate();
            }
        }
    }

/* WEBPACK VAR INJECTION */}.call(this, __webpack_require__(/*! ./../node_modules/next/dist/compiled/webpack/harmony-module.js */ "./node_modules/next/dist/compiled/webpack/harmony-module.js")(module)))

/***/ })

})
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9fTl9FLy4vcGFnZXMvaW5kZXgudHN4Il0sIm5hbWVzIjpbIkhvbWUiLCJwcm9wcyIsInN0YXRlIiwibGF5b3V0IiwiZGVmYXVsdF9yb29tX3N0YXRlIiwibm9fc2VhdF9yb29tX3N0YXRlIiwiY29tcG9uZW50IiwiaW50ZXJ2YWxJZCIsInNldEludGVydmFsIiwiZmV0Y2hlciIsInRoZW4iLCJyIiwiZGVmYXVsdF9yb29tIiwic2VhdHMiLCJmb3JFYWNoIiwiaXRlbSIsImNvbnNvbGUiLCJsb2ciLCJzZWF0X2lkIiwidXNlcl9kaXNwbGF5X25hbWUiLCJzZXRTdGF0ZSIsImRlZmF1bHRfcm9vbV9sYXlvdXQiLCJub19zZWF0X3Jvb20iLCJlcnIiLCJlcnJvciIsImNsZWFySW50ZXJ2YWwiLCJoZWlnaHQiLCJ3aWR0aCIsIm1hcmdpbiIsInBvc2l0aW9uIiwiUmVhY3QiLCJDb21wb25lbnQiXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQUFBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7SUFHcUJBLEk7Ozs7O0FBR25CLGdCQUFZQyxLQUFaLEVBQXdCO0FBQUE7O0FBQUE7O0FBQ3RCLDhCQUFNQSxLQUFOOztBQURzQjs7QUFFdEIsVUFBS0MsS0FBTCxHQUFhO0FBQ1hDLFlBQU0sRUFBRSxJQURHO0FBRVhDLHdCQUFrQixFQUFFLElBRlQ7QUFHWEMsd0JBQWtCLEVBQUU7QUFIVCxLQUFiO0FBRnNCO0FBT3ZCOzs7O3dDQUVtQjtBQUNsQixVQUFNQyxTQUFTLEdBQUcsSUFBbEI7QUFDQSxXQUFLQyxVQUFMLEdBQWtCQyxXQUFXLENBQUMsWUFBTTtBQUNsQ0MscUVBQU8sMkVBQVAsQ0FHR0MsSUFISCxDQUdRLFVBQUNDLENBQUQsRUFBTztBQUNYQSxXQUFDLENBQUNDLFlBQUYsQ0FBZUMsS0FBZixDQUFxQkMsT0FBckIsQ0FBNkIsVUFBQ0MsSUFBRDtBQUFBLG1CQUMzQkMsT0FBTyxDQUFDQyxHQUFSLENBQVlGLElBQUksQ0FBQ0csT0FBakIsRUFBMEJILElBQUksQ0FBQ0ksaUJBQS9CLENBRDJCO0FBQUEsV0FBN0I7QUFHQUgsaUJBQU8sQ0FBQ0MsR0FBUixDQUFZLFNBQVo7QUFDQVgsbUJBQVMsQ0FBQ2MsUUFBVixDQUFtQjtBQUNqQmpCLGtCQUFNLEVBQUVRLENBQUMsQ0FBQ1UsbUJBRE87QUFFakJqQiw4QkFBa0IsRUFBRU8sQ0FBQyxDQUFDQyxZQUZMO0FBR2pCUCw4QkFBa0IsRUFBRU0sQ0FBQyxDQUFDVztBQUhMLFdBQW5CO0FBS0QsU0FiSCxXQWNTLFVBQUNDLEdBQUQ7QUFBQSxpQkFBU1AsT0FBTyxDQUFDUSxLQUFSLENBQWNELEdBQWQsQ0FBVDtBQUFBLFNBZFQ7QUFlRCxPQWhCNEIsRUFnQjFCLElBaEIwQixDQUE3QjtBQWlCRDs7OzJDQUVzQjtBQUNyQixVQUFJLEtBQUtoQixVQUFULEVBQXFCO0FBQ25Ca0IscUJBQWEsQ0FBQyxLQUFLbEIsVUFBTixDQUFiO0FBQ0Q7QUFDRjs7OzZCQUVRO0FBQ1AsMEJBQ0U7QUFDRSxhQUFLLEVBQUU7QUFDTG1CLGdCQUFNLEVBQUUsSUFESDtBQUVMQyxlQUFLLEVBQUUsSUFGRjtBQUdMQyxnQkFBTSxFQUFFLENBSEg7QUFJTEMsa0JBQVEsRUFBRTtBQUpMLFNBRFQ7QUFBQSxnQ0FRRSxxRUFBQyx5REFBRDtBQUFBO0FBQUE7QUFBQTtBQUFBLGdCQVJGLGVBU0UscUVBQUMsNERBQUQ7QUFDRSw0QkFBa0IsRUFBRSxLQUFLM0IsS0FBTCxDQUFXRSxrQkFEakM7QUFFRSw0QkFBa0IsRUFBRSxLQUFLRixLQUFMLENBQVdHO0FBRmpDO0FBQUE7QUFBQTtBQUFBO0FBQUEsZ0JBVEYsZUFhRSxxRUFBQyxnRUFBRDtBQUNFLGdCQUFNLEVBQUUsS0FBS0gsS0FBTCxDQUFXQyxNQURyQjtBQUVFLG1CQUFTLEVBQUUsS0FBS0QsS0FBTCxDQUFXRTtBQUZ4QjtBQUFBO0FBQUE7QUFBQTtBQUFBLGdCQWJGLGVBaUJFLHFFQUFDLGlFQUFEO0FBQWMsNEJBQWtCLEVBQUUsS0FBS0YsS0FBTCxDQUFXRztBQUE3QztBQUFBO0FBQUE7QUFBQTtBQUFBLGdCQWpCRjtBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUEsY0FERjtBQXFCRDs7OztFQTdEK0J5Qiw0Q0FBSyxDQUFDQyxTIiwiZmlsZSI6InN0YXRpYy93ZWJwYWNrL3BhZ2VzL2luZGV4LjczMzlkMmQzZjJmMjk0ZWQxMTk5LmhvdC11cGRhdGUuanMiLCJzb3VyY2VzQ29udGVudCI6WyJpbXBvcnQgUmVhY3QgZnJvbSBcInJlYWN0XCI7XG5pbXBvcnQgQ2xvY2sgZnJvbSBcIi4uL2NvbXBvbmVudHMvQ2xvY2tcIjtcbmltcG9ydCBNZXNzYWdlIGZyb20gXCIuLi9jb21wb25lbnRzL01lc3NhZ2VcIjtcbmltcG9ydCBEZWZhdWx0Um9vbSBmcm9tIFwiLi4vY29tcG9uZW50cy9EZWZhdWx0Um9vbVwiO1xuaW1wb3J0IFN0YW5kaW5nUm9vbSBmcm9tIFwiLi4vY29tcG9uZW50cy9TdGFuZGluZ1Jvb21cIjtcbmltcG9ydCBmZXRjaGVyIGZyb20gXCIuLi9saWIvZmV0Y2hlclwiO1xuaW1wb3J0IHsgUm9vbXNTdGF0ZVJlc3BvbnNlLCBzZWF0IH0gZnJvbSBcIi4uL3R5cGVzL3Jvb20tc3RhdGVcIjtcblxuZXhwb3J0IGRlZmF1bHQgY2xhc3MgSG9tZSBleHRlbmRzIFJlYWN0LkNvbXBvbmVudDx7fSwgYW55PiB7XG4gIHByaXZhdGUgaW50ZXJ2YWxJZDogTm9kZUpTLlRpbWVvdXQgfCB1bmRlZmluZWQ7XG5cbiAgY29uc3RydWN0b3IocHJvcHM6IGFueSkge1xuICAgIHN1cGVyKHByb3BzKTtcbiAgICB0aGlzLnN0YXRlID0ge1xuICAgICAgbGF5b3V0OiBudWxsLFxuICAgICAgZGVmYXVsdF9yb29tX3N0YXRlOiBudWxsLFxuICAgICAgbm9fc2VhdF9yb29tX3N0YXRlOiBudWxsLFxuICAgIH07XG4gIH1cblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICBjb25zdCBjb21wb25lbnQgPSB0aGlzO1xuICAgIHRoaXMuaW50ZXJ2YWxJZCA9IHNldEludGVydmFsKCgpID0+IHtcbiAgICAgIGZldGNoZXI8Um9vbXNTdGF0ZVJlc3BvbnNlPihcbiAgICAgICAgYGh0dHBzOi8vdGFhNHA5a2xoYS5leGVjdXRlLWFwaS5hcC1ub3J0aGVhc3QtMS5hbWF6b25hd3MuY29tL3Jvb21zX3N0YXRlYFxuICAgICAgKVxuICAgICAgICAudGhlbigocikgPT4ge1xuICAgICAgICAgIHIuZGVmYXVsdF9yb29tLnNlYXRzLmZvckVhY2goKGl0ZW06IHNlYXQpID0+XG4gICAgICAgICAgICBjb25zb2xlLmxvZyhpdGVtLnNlYXRfaWQsIGl0ZW0udXNlcl9kaXNwbGF5X25hbWUpXG4gICAgICAgICAgKTtcbiAgICAgICAgICBjb25zb2xlLmxvZyhcImZldGNo5a6M5LqGXCIpO1xuICAgICAgICAgIGNvbXBvbmVudC5zZXRTdGF0ZSh7XG4gICAgICAgICAgICBsYXlvdXQ6IHIuZGVmYXVsdF9yb29tX2xheW91dCxcbiAgICAgICAgICAgIGRlZmF1bHRfcm9vbV9zdGF0ZTogci5kZWZhdWx0X3Jvb20sXG4gICAgICAgICAgICBub19zZWF0X3Jvb21fc3RhdGU6IHIubm9fc2VhdF9yb29tLFxuICAgICAgICAgIH0pO1xuICAgICAgICB9KVxuICAgICAgICAuY2F0Y2goKGVycikgPT4gY29uc29sZS5lcnJvcihlcnIpKTtcbiAgICB9LCAxNTAwKTtcbiAgfVxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIGlmICh0aGlzLmludGVydmFsSWQpIHtcbiAgICAgIGNsZWFySW50ZXJ2YWwodGhpcy5pbnRlcnZhbElkKTtcbiAgICB9XG4gIH1cblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXZcbiAgICAgICAgc3R5bGU9e3tcbiAgICAgICAgICBoZWlnaHQ6IDEwODAsXG4gICAgICAgICAgd2lkdGg6IDE5MjAsXG4gICAgICAgICAgbWFyZ2luOiAwLFxuICAgICAgICAgIHBvc2l0aW9uOiBcInJlbGF0aXZlXCIsXG4gICAgICAgIH19XG4gICAgICA+XG4gICAgICAgIDxDbG9jayAvPlxuICAgICAgICA8TWVzc2FnZVxuICAgICAgICAgIGRlZmF1bHRfcm9vbV9zdGF0ZT17dGhpcy5zdGF0ZS5kZWZhdWx0X3Jvb21fc3RhdGV9XG4gICAgICAgICAgbm9fc2VhdF9yb29tX3N0YXRlPXt0aGlzLnN0YXRlLm5vX3NlYXRfcm9vbV9zdGF0ZX1cbiAgICAgICAgLz5cbiAgICAgICAgPERlZmF1bHRSb29tXG4gICAgICAgICAgbGF5b3V0PXt0aGlzLnN0YXRlLmxheW91dH1cbiAgICAgICAgICByb29tU3RhdGU9e3RoaXMuc3RhdGUuZGVmYXVsdF9yb29tX3N0YXRlfVxuICAgICAgICAvPlxuICAgICAgICA8U3RhbmRpbmdSb29tIG5vX3NlYXRfcm9vbV9zdGF0ZT17dGhpcy5zdGF0ZS5ub19zZWF0X3Jvb21fc3RhdGV9IC8+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59XG4iXSwic291cmNlUm9vdCI6IiJ9