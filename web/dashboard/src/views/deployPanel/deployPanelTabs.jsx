function classNames(...classes) {
  return classes.filter(Boolean).join(' ')
}

export default function DeployPanelTabs(tabs, switchTab) {
  return (
    <div className="">
      <div className="sm:hidden">
        <label htmlFor="tabs" className="sr-only">
          Select a tab
        </label>
        {/* Use an "onChange" listener to redirect the user to the selected tab URL. */}
        <select
          id="tabs"
          name="tabs"
          className="block w-full rounded-md border-gray-300 pl-3 pr-10 text-base focus:border-indigo-500 focus:outline-none focus:ring-indigo-500 sm:text-sm"
          defaultValue={tabs.find((tab) => tab.current).name}
        >
          {tabs.map((tab) => (
            <option key={tab.name}>{tab.name}</option>
          ))}
        </select>
      </div>
      <div className="hidden sm:block">
        <div>
          <nav className="flex space-x-8" aria-label="Tabs">
            {tabs.map((tab) => (
              <span
                key={tab.name}
                onClick={() => {switchTab(tab.name); return false}}
                className={classNames(
                  tab.current
                    ? 'border-gray-300 text-gray-300'
                    : 'border-transparent text-gray-400 hover:border-gray-300 hover:text-gray-300',
                  'whitespace-nowrap border-b-2 pb-2 px-1 text-sm font-medium cursor-pointer'
                )}
                aria-current={tab.current ? 'page' : undefined}
              >
                {tab.name}
              </span>
            ))}
          </nav>
        </div>
      </div>
    </div>
  )
}
