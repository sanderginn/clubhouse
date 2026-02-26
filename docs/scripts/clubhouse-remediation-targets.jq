def location_path: .location.path // .file // "";
def location_symbol: .location.symbol // .function // "";
def location_line: ((.location.line // .line // 0) | tonumber? // 0);

[
  .violations[]
  | {
      rule,
      severity,
      message,
      path: location_path,
      symbol: location_symbol,
      line: location_line,
      fix
    }
  | select(.path != "" and .symbol != "" and (.line > 0))
]
