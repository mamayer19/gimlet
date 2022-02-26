import { useState } from 'react'
import { Switch } from '@headlessui/react'
import { InformationCircleIcon } from '@heroicons/react/solid'

function classNames(...classes) {
  return classes.filter(Boolean).join(' ')
}

const EnvironmentCard = ({ isOnline, singleEnv, deleteEnv, hasGitopsRepo }) => {
  const [enabled, setEnabled] = useState(false)

  const createGitopsLink = (url, name) => {
    return (
      <a className="cursor-pointer text-gray-500 hover:text-gray-700 mr-4"
        target="_blank"
        rel="noreferrer"
        href={url}>
        {name}
      </a>
    )
  }

  const gitopsBootstrapWizard = () => {
    return (
      <>
        <div className="mt-2 pb-4 border-b border-gray-200">
          <h3 className="text-lg leading-6 font-medium text-gray-900">Bootstrap gitops repository</h3>
          <p className="mt-2 max-w-4xl text-sm text-gray-500">
            To initialize this environment, bootstrap the gitops repository first
          </p>
        </div>

        <div className="mt-4 rounded-md bg-blue-50 p-4">
          <div className="flex">
            <div className="flex-shrink-0">
              <InformationCircleIcon className="h-5 w-5 text-blue-400" aria-hidden="true" />
            </div>
            <div className="ml-3 md:justify-between">
              <p className="text-sm text-blue-500">
                By default manifests of this environment will be placed in the <span className="text-xs font-mono bg-blue-100 text-blue-500 font-medium px-1 py-1 rounded">{singleEnv.name}</span> folder in the shared <span className="text-xs font-mono bg-blue-100 font-medium text-blue-500 px-1 py-1 rounded">gitops-infra</span> git repository
              </p>
            </div>
          </div>
        </div>

        <div className=" text-gray-700">
          <div>
            <div className="sm:pt-5 flex">
              <label htmlFor="username" className="block text-sm text-gray-700 sm:mt-px sm:pt-2 sm:col-span-2">
                <div className="font-medium">Separate environments by git repositories</div>
              </label>
              <div className="mt-1 ml-4 sm:mt-0">
                <div className="max-w-lg flex rounded-md">
                  <Switch
                    checked={enabled}
                    onChange={setEnabled}
                    className={classNames(
                      enabled ? 'bg-indigo-600' : 'bg-gray-200',
                      'relative inline-flex flex-shrink-0 h-6 w-11 border-2 border-transparent rounded-full cursor-pointer transition-colors ease-in-out duration-200'
                    )}
                  >
                    <span className="sr-only">Use setting</span>
                    <span
                      aria-hidden="true"
                      className={classNames(
                        enabled ? 'translate-x-5' : 'translate-x-0',
                        'pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform ring-0 transition ease-in-out duration-200'
                      )}
                    />
                  </Switch>
                </div>
              </div>
            </div>

          </div>
          <div className="mt-2 text-sm text-gray-500 leading-loose">Manifests will be placed in the environment specific <span className="text-xs font-mono bg-gray-100 text-gray-500 font-medium px-1 py-1 rounded">gitops-{singleEnv.name}-infra</span> repository</div>
          <div className="p-0 flow-root mt-8">
            <span className="inline-flex rounded-md shadow-sm gap-x-3 float-right">
              <button
                // disabled={this.state.input === "" || this.state.saveButtonTriggered}
                onClick={() => console.log("MEGNYOMTAK")}
                className="bg-green-600 hover:bg-green-500 focus:outline-none focus:border-green-700 focus:shadow-outline-indigo active:bg-green-700 inline-flex items-center px-6 py-3 border border-transparent text-base leading-6 font-medium rounded-md text-white transition ease-in-out duration-150"
              >
                Bootstrap gitops repository
              </button>
            </span>
          </div>
        </div>
      </>
    )
  }

  return (
    <div className="my-4 bg-white overflow-hidden shadow rounded-lg divide-y divide-gray-200">
      <div className="px-4 py-5 sm:px-6">
        <div className="flex justify-between">
          <div className="inline-flex">
            <h3 className="text-lg leading-6 font-medium text-gray-900 pr-1">
              {singleEnv.name}
            </h3>
            <span title={isOnline ? "Connected" : "Disconnected"}>
              <svg className={(isOnline ? "text-green-400" : "text-red-400") + " inline fill-current ml-1"} xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 20 20">
                <path
                  d="M0 14v1.498c0 .277.225.502.502.502h.997A.502.502 0 0 0 2 15.498V14c0-.959.801-2.273 2-2.779V9.116C1.684 9.652 0 11.97 0 14zm12.065-9.299l-2.53 1.898c-.347.26-.769.401-1.203.401H6.005C5.45 7 5 7.45 5 8.005v3.991C5 12.55 5.45 13 6.005 13h2.327c.434 0 .856.141 1.203.401l2.531 1.898a3.502 3.502 0 0 0 2.102.701H16V4h-1.832c-.758 0-1.496.246-2.103.701zM17 6v2h3V6h-3zm0 8h3v-2h-3v2z"
                />
              </svg>
            </span>
            {!hasGitopsRepo &&
              <span title="Uninitiated">
                <svg xmlns="http://www.w3.org/2000/svg" className="ml-1 h-6 w-6 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </span>}
          </div>
          {!isOnline &&
            <div className="inline-flex">
              <a className="cursor-pointer text-gray-500 hover:text-gray-700 mr-4"
                target="_blank"
                rel="noreferrer"
                href="https://gimlet.io/docs/installing-gimlet-agent">
                Install agent
              </a>
              <svg xmlns="http://www.w3.org/2000/svg" onClick={deleteEnv} className="cursor-pointer inline text-red-400 hover:text-red-600 h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </div>
          }
        </div>
      </div>
      <div className="px-4 py-5 sm:px-6">
        {hasGitopsRepo ?
          <div className="inline-flex">
            {createGitopsLink("https://gimlet.io/docs/installing-gimlet-agent", "Gitops-infra")}
            {createGitopsLink("https://gimlet.io/docs/installing-gimlet-agent", "Gitops-apps")}
          </div>
          :
          gitopsBootstrapWizard()
        }
      </div>
    </div >
  )
};

export default EnvironmentCard;
